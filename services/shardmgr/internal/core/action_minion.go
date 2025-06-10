package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

type ActionMinion struct {
	ss        *ServiceState
	moveState *MoveState
}

func NewActionMinion(ctx context.Context, ss *ServiceState, moveState *MoveState) *ActionMinion {
	minion := &ActionMinion{
		ss:        ss,
		moveState: moveState,
	}
	go minion.Run(ctx, ss)
	return minion
}

func (am *ActionMinion) Run(ctx context.Context, ss *ServiceState) {
	startMs := kcommon.GetWallTimeMs()
	ke := kcommon.TryCatchRun(ctx, func() {
		am.run(ctx)
	})
	elapseMs := kcommon.GetWallTimeMs() - startMs
	if ke != nil {
		costfunc.ProposalFailMetrics.GetTimeSequence(ctx, am.moveState.SolverType).Add(1)
		klogging.Error(ctx).WithError(ke).With("signature", am.moveState.Signature).With("proposalId", am.moveState.ProposalId).With("elapsedMs", elapseMs).Log("ActionMinion.Run", "Error")
	} else {
		costfunc.ProposalSuccMetrics.GetTimeSequence(ctx, am.moveState.SolverType).Add(1)
		klogging.Info(ctx).With("signature", am.moveState.Signature).With("proposalId", am.moveState.ProposalId).With("elapsedMs", elapseMs).Log("ActionMinion.Run", "done")
	}
	ss.actionProvider.StoreActionNode(ctx, am.moveState.ProposalId, nil)
	ss.VisitServiceState(ctx, func(ss *ServiceState) {
		delete(ss.AllMoves, am.moveState.ProposalId)
	})
}

func (am *ActionMinion) run(ctx context.Context) {
	klogging.Info(ctx).With("signature", am.moveState.Signature).With("proposalId", am.moveState.ProposalId).With("actionCount", len(am.moveState.Actions)).Log("ActionMinion", "Started")
	for am.moveState.CurrentAction < len(am.moveState.Actions) {
		action := am.moveState.Actions[am.moveState.CurrentAction]
		klogging.Info(ctx).With("action", action).With("proposalId", am.moveState.ProposalId).Log("ActionMinion", "RunAction")
		switch action.ActionType {
		case smgjson.AT_RemoveFromRoutingAndSleep:
			am.actionRemoveFromRouting(ctx, am.moveState.CurrentAction)
		case smgjson.AT_AddToRouting:
			am.actionAddToRouting(ctx, am.moveState.CurrentAction)
		case smgjson.AT_AddShard:
			am.actionAddShard(ctx, am.moveState.CurrentAction)
		case smgjson.AT_DropShard:
			am.actionDropShard(ctx, am.moveState.CurrentAction)
		default:
			panic(kerror.Create("UnknownActionType", "unknown action type").With("proposalId", am.moveState.ProposalId).With("actionType", action.ActionType))
		}
		am.moveState.CurrentAction++
	}
}

// in case anything goes wrong, we will panic
func (am *ActionMinion) actionRemoveFromRouting(ctx context.Context, stepIdx int) {
	action := am.moveState.Actions[stepIdx]
	if action.ActionStage == smgjson.AS_NotStarted {
		workerFullId := action.From
		// step 1: remove from routing table
		succ := true
		var ke *kerror.Kerror
		am.ss.PostActionAndWait(func(ss *ServiceState) {
			ws := am.ss.FindWorkerStateByWorkerFullId(workerFullId)
			if ws == nil {
				succ = false
				ke = kerror.Create("SrcWorkerNotFound", "worker not found").With("workerFullId", workerFullId).With("proposalId", am.moveState.ProposalId)
				return
			}
			assign, ok := ss.AllAssignments[action.SrcAssignmentId]
			if !ok {
				succ = false
				ke = kerror.Create("AssignmentNotFound", "assignment not found").With("assignmentId", action.SrcAssignmentId).With("proposalId", am.moveState.ProposalId)
				return
			}
			// update routing table
			assign.ShouldInRoutingTable = false
			ss.FlushWorkerState(ctx, workerFullId, ws, FS_WorkerState|FS_Routing, "removeFromRouting")
		}, "actionRemoveFromRouting")
		if !succ {
			panic(ke)
		}
		action.ActionStage = smgjson.AS_Conducted
		am.ss.actionProvider.StoreActionNode(ctx, am.moveState.ProposalId, am.moveState.ToMoveStateJson("removeFromRouting"))
	}

	// step 2: sleep
	if action.SleepMs > 0 {
		kcommon.SleepMs(ctx, action.SleepMs)
	}
	action.ActionStage = smgjson.AS_Completed
}

func (am *ActionMinion) actionAddToRouting(ctx context.Context, stepIdx int) {
	status := AS_Unknown
	var ke *kerror.Kerror
	action := am.moveState.Actions[stepIdx]
	if action.ActionStage == smgjson.AS_NotStarted {
		workerFullId := action.To
		// step 1: add to routing table
		am.ss.PostActionAndWait(func(ss *ServiceState) {
			ws := am.ss.FindWorkerStateByWorkerFullId(workerFullId)
			if ws == nil {
				ke = kerror.Create("DestWorkerNotFound", "worker not found").With("workerFullId", workerFullId).With("proposalId", am.moveState.ProposalId)
				status = AS_Failed
				return
			}
			assign, ok := ss.AllAssignments[action.DestAssignmentId]
			if !ok {
				ke = kerror.Create("AssignmentNotFound", "assignment not found").With("assignmentId", action.DestAssignmentId).With("proposalId", am.moveState.ProposalId)
				status = AS_Failed
				return
			}
			// update routing table
			assign.ShouldInRoutingTable = true
			ss.FlushWorkerState(ctx, workerFullId, ws, FS_WorkerState|FS_Routing, "addToRouting")
			status = AS_Completed
		}, "actionAddToRouting")
		action.ActionStage = smgjson.AS_Completed
		am.ss.actionProvider.StoreActionNode(ctx, am.moveState.ProposalId, am.moveState.ToMoveStateJson("addToRouting"))
	}
	klogging.Info(ctx).With("proposalId", am.moveState.ProposalId).With("worker", action.To).With("wallTime", kcommon.GetWallTimeMs()).With("status", status).With("stage", action.ActionStage).Log("actionAddToRouting", "add to routing table")
	if status == AS_Failed {
		panic(ke)
	}
}

type ActionStatus string

const (
	AS_Unknown   ActionStatus = "Unknown"
	AS_Wait      ActionStatus = "Wait"
	AS_Completed ActionStatus = "Completed"
	AS_Failed    ActionStatus = "Failed"
)

func (am *ActionMinion) actionAddShard(ctx context.Context, stepIdx int) {
	status := AS_Unknown
	var signalBox *SignalBox
	action := am.moveState.Actions[stepIdx]
	// step 1: create shard
	if action.ActionStage == smgjson.AS_NotStarted {
		var ke *kerror.Kerror
		workerFullId := action.To
		am.ss.PostActionAndWait(func(ss *ServiceState) {
			// // snapshot (add replica)
			// snapshot := ss.GetSnapshotCurrentForClone().Clone()
			// {
			// 	shardSnap := snapshot.AllShards.GetOrPanic(action.ShardId)
			// 	replicaSnap := shardSnap.Replicas[action.DestReplicaIdx]
			// 	if replicaSnap == nil {
			// 		replicaSnap = costfunc.NewReplicaSnap(action.ShardId, action.DestReplicaIdx)
			// 		shardSnap.Replicas[action.DestReplicaIdx] = replicaSnap
			// 		newShardSnap := shardSnap.Clone()
			// 		newShardSnap.Replicas[action.DestReplicaIdx] = replicaSnap
			// 		snapshot.AllShards.Set(action.ShardId, newShardSnap)
			// 	}
			// }

			// ss
			shardId := data.ShardId(action.ShardId)
			shardState, ok := ss.AllShards[shardId]
			if !ok {
				ke = kerror.Create("ShardNotFound", "shard not found").With("shardId", shardId).With("proposalId", am.moveState.ProposalId)
				status = AS_Failed
				return
			}
			_, ok = ss.AllAssignments[action.DestAssignmentId]
			if ok {
				ke = kerror.Create("AssignmentAlreadyExist", "assignment already exist").With("assignmentId", action.DestAssignmentId).With("proposalId", am.moveState.ProposalId)
				status = AS_Failed
				return
			}
			workerState := ss.FindWorkerStateByWorkerFullId(workerFullId)
			if workerState == nil {
				ke = kerror.Create("DestWorkerNotFound", "worker not found").With("workerFullId", workerFullId).With("proposalId", am.moveState.ProposalId)
				status = AS_Failed
				return
			}
			replicaState, ok := shardState.Replicas[action.DestReplicaIdx]
			if !ok {
				// replica not exist yet, create it
				replicaState = NewReplicaState(action.ShardId, action.DestReplicaIdx)
				shardState.Replicas[action.DestReplicaIdx] = replicaState
			}
			// commit to snapshot+ss
			// ss.SetSnapshotCurrent(ctx, snapshot.Freeze(), "minion add shard")
			assign := NewAssignmentState(action.DestAssignmentId, shardId, action.DestReplicaIdx, workerFullId)
			assign.TargetState = cougarjson.CAS_Ready
			assign.ShouldInPilot = true
			ss.AllAssignments[action.DestAssignmentId] = assign
			replicaState.Assignments[action.DestAssignmentId] = common.Unit{}
			workerState.Assignments[action.ShardId] = action.DestAssignmentId
			ss.storeProvider.StoreShardState(shardId, shardState.ToJson())
			ss.FlushWorkerState(ctx, workerFullId, workerState, FS_WorkerState|FS_Pilot, "addShard")
			// wait on signal box
			signalBox = workerState.SignalBox
			status = AS_Wait
			klogging.Info(ctx).With("proposalId", am.moveState.ProposalId).With("worker", action.To).With("wallTime", kcommon.GetWallTimeMs()).With("status", status).With("assignmentId", action.DestAssignmentId).Log("actionAddShard", "add shard to worker")
		}, "actionAddShard")
		if status == AS_Failed {
			panic(ke)
		}
		action.ActionStage = smgjson.AS_Conducted
		am.ss.actionProvider.StoreActionNode(ctx, am.moveState.ProposalId, am.moveState.ToMoveStateJson("addShard"))
	}
	// step 2: wail until this assignment is ready (based on feedback from ephemeral node)
	klogging.Info(ctx).With("proposalId", am.moveState.ProposalId).With("worker", action.To).With("wallTime", kcommon.GetWallTimeMs()).With("status", status).Log("actionAddShard", "wait for assignment to be ready")
	var ke *kerror.Kerror
	for status == AS_Wait {
		loopId := kcommon.RandomString(ctx, 8)
		if signalBox != nil {
			select {
			case <-ctx.Done():
				status = AS_Failed
				panic(kerror.Create("ContextCanceled", "context canceled"))
			case <-signalBox.NotifyCh:
				klogging.Info(ctx).With("proposalId", am.moveState.ProposalId).With("worker", action.To).With("wallTime", kcommon.GetWallTimeMs()).With("notifyReason", signalBox.NotifyReason).With("notifyId", signalBox.NofityId).With("loopId", loopId).Log("actionAddShard", "wake up")
			}
		}
		var reason string
		am.ss.PostActionAndWait(func(ss *ServiceState) {
			// ss
			workerState, ok := ss.AllWorkers[action.To]
			if !ok {
				ke = kerror.Create("DestWorkerNotFound", "worker not found").With("workerFullId", action.To)
				status = AS_Failed
				return
			}
			if !workerState.IsGoodMoveTarget() {
				ke = kerror.Create("DestWorkerNotGood", "worker not suitable as target").With("workerFullId", action.To).With("state", workerState.State)
				status = AS_Failed
				return
			}
			signalBox = workerState.SignalBox
			assign, ok := ss.AllAssignments[action.DestAssignmentId]
			if !ok {
				reason = "assignment not found"
				return
			}
			if assign.CurrentConfirmedState == cougarjson.CAS_Ready {
				status = AS_Completed
				// apply to current snapshot
				ke = kcommon.TryCatchRun(ctx, func() {
					ss.ModifySnapshotCurrent(ctx, func(snap *costfunc.Snapshot) {
						action.ApplyToSnapshot(snap, costfunc.AM_Strict)
					}, action.String())
				})
				if ke != nil {
					reason = "apply to snapshot failed:" + ke.FullString()
					status = AS_Failed
					return
				}
				return
			}
			reason = "assignment not in ready state"
		}, "actionAddShardWait")

		klogging.Info(ctx).With("proposalId", am.moveState.ProposalId).With("worker", action.To).With("wallTime", kcommon.GetWallTimeMs()).With("loopId", loopId).With("reason", reason).With("status", status).WithError(ke).Log("actionAddShard", "loop end")
	}
	if status == AS_Failed {
		panic(ke)
	}
	action.ActionStage = smgjson.AS_Completed
	am.ss.actionProvider.StoreActionNode(ctx, am.moveState.ProposalId, am.moveState.ToMoveStateJson("addShardDone"))
}

func (workerState *WorkerState) IsGoodMoveTarget() bool {
	switch workerState.State {
	case data.WS_Online_healthy, data.WS_Online_shutdown_req:
		return true
	case data.WS_Offline_graceful_period, data.WS_Offline_draining_candidate:
		return true
	default:
		return false
	}
}

func (am *ActionMinion) actionDropShard(ctx context.Context, stepIdx int) {
	status := AS_Unknown
	var signalBox *SignalBox
	var ke *kerror.Kerror
	action := am.moveState.Actions[stepIdx]
	// step 1: drop shard
	if action.ActionStage == smgjson.AS_NotStarted {
		workerFullId := action.From
		am.ss.PostActionAndWait(func(ss *ServiceState) {
			shardId := data.ShardId(action.ShardId)
			shardState, ok := ss.AllShards[shardId]
			if !ok {
				ke = kerror.Create("ShardNotFound", "shard not found").With("shardId", shardId)
				status = AS_Failed
				return
			}
			replicaState, ok := shardState.Replicas[action.SrcReplicaIdx]
			if !ok {
				ke = kerror.Create("ReplicaNotFound", "replica not found").With("shardId", shardId).With("replicaIdx", action.SrcReplicaIdx)
				status = AS_Failed
				return
			}
			if _, ok := replicaState.Assignments[action.SrcAssignmentId]; !ok {
				ke = kerror.Create("AssignmentNotFound", "assignment not found in replica").With("assignmentId", action.SrcAssignmentId)
				status = AS_Failed
				return
			}
			assign, ok := ss.AllAssignments[action.SrcAssignmentId]
			if !ok {
				ke = kerror.Create("AssignmentNotFound", "assignment not found").With("assignmentId", action.SrcAssignmentId)
				status = AS_Failed
				return
			}
			workerState := ss.FindWorkerStateByWorkerFullId(workerFullId)
			if workerState == nil {
				ke = kerror.Create("SrcWorkerNotFound", "worker not found").With("workerFullId", workerFullId)
				status = AS_Failed
				return
			}
			assign.ShouldInPilot = false
			assign.TargetState = cougarjson.CAS_Dropped
			if action.DeleteReplica {
				replicaState.LameDuck = true
			}
			ss.storeProvider.StoreShardState(shardId, shardState.ToJson())
			ss.FlushWorkerState(ctx, workerFullId, workerState, FS_Most, "dropShard")
			if workerState.IsOnline() { // in case worker is not online or already dropped, we don't need to wait for signal box
				if assign.CurrentConfirmedState != cougarjson.CAS_Dropped && assign.CurrentConfirmedState != cougarjson.CAS_Unknown {
					signalBox = workerState.SignalBox
				}
			}
			status = AS_Wait
		}, "actionDropShard")
		if status == AS_Failed {
			panic(ke)
		}
		action.ActionStage = smgjson.AS_Conducted
		am.ss.actionProvider.StoreActionNode(ctx, am.moveState.ProposalId, am.moveState.ToMoveStateJson("dropShard"))
	}
	// step 2: wail until drop assignment is completed (based on feedback from ephemeral node)
	for status == AS_Wait {
		if signalBox != nil {
			select {
			case <-ctx.Done():
				panic(kerror.Create("ContextCanceled", "context canceled"))
			case <-signalBox.NotifyCh:
				klogging.Info(ctx).With("proposalId", am.moveState.ProposalId).With("worker", action.From).With("wallTime", kcommon.GetWallTimeMs()).With("notifyReason", signalBox.NotifyReason).With("notifyId", signalBox.NofityId).Log("actionDropShard", "wake up")
			}
		}
		am.ss.PostActionAndWait(func(ss *ServiceState) {
			applyToSnapshot := func() {
				// remove from current snapshot
				ke = kcommon.TryCatchRun(ctx, func() {
					ss.ModifySnapshotCurrent(ctx, func(snap *costfunc.Snapshot) {
						action.ApplyToSnapshot(snap, costfunc.AM_Relaxed)
					}, action.String())
					// action.ApplyToSnapshot(ss.GetSnapshotCurrentForModify(ctx, action.String()), costfunc.AM_Relaxed)
				})
				if ke != nil {
					status = AS_Failed
				}
			}
			workerState, ok := ss.AllWorkers[action.From]
			if !ok {
				status = AS_Completed
				applyToSnapshot()
				return
			}
			assign, ok := ss.AllAssignments[action.SrcAssignmentId]
			if !ok {
				status = AS_Completed
				applyToSnapshot()
				return
			}
			if !workerState.IsOnline() {
				assign.CurrentConfirmedState = cougarjson.CAS_Dropped
				status = AS_Completed
				applyToSnapshot()
				return
			}
			if assign.CurrentConfirmedState == cougarjson.CAS_Dropped || assign.CurrentConfirmedState == cougarjson.CAS_Unknown {
				status = AS_Completed
				applyToSnapshot()
				return
			}
			signalBox = workerState.SignalBox
		}, "actionDropShardWait")
	}
	if status == AS_Failed {
		panic(ke)
	}
	action.ActionStage = smgjson.AS_Completed
	am.ss.actionProvider.StoreActionNode(ctx, am.moveState.ProposalId, am.moveState.ToMoveStateJson("dropShardDone"))
}

// ActionEvent implements IEvent
type ActionEvent struct {
	createTimeMs int64 // time when the event was created
	name         string
	fn           func(ss *ServiceState)
}

func NewActionEvent(fn func(ss *ServiceState), name string) *ActionEvent {
	return &ActionEvent{
		createTimeMs: kcommon.GetWallTimeMs(),
		fn:           fn,
		name:         name,
	}
}

func (ae *ActionEvent) GetCreateTimeMs() int64 {
	return ae.createTimeMs
}
func (ae *ActionEvent) GetName() string {
	return ae.name
}

func (ae *ActionEvent) Process(ctx context.Context, ss *ServiceState) {
	ae.fn(ss)
}
