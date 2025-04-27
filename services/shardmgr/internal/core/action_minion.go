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
		klogging.Error(ctx).WithError(ke).With("signature", am.moveState.Signature).With("proposalId", am.moveState.ProposalId).With("elapsedMs", elapseMs).Log("ActionMinion.Run", "Error")
	} else {
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
		klogging.Info(ctx).With("action", action).Log("ActionMinion", "RunAction")
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
			panic(kerror.Create("UnknownActionType", "unknown action type").With("actionType", action.ActionType))
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
				ke = kerror.Create("SrcWorkerNotFound", "worker not found").With("workerFullId", workerFullId)
				return
			}
			assign, ok := ss.AllAssignments[action.AssignmentId]
			if !ok {
				succ = false
				ke = kerror.Create("AssignmentNotFound", "assignment not found").With("assignmentId", action.AssignmentId)
				return
			}
			// update routing table
			assign.ShouldInRoutingTable = false
			ss.FlushWorkerState(ctx, workerFullId, ws, FS_WorkerState|FS_Routing, "removeFromRouting")
		})
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
				ke = kerror.Create("DestWorkerNotFound", "worker not found").With("workerFullId", workerFullId)
				status = AS_Failed
				return
			}
			assign, ok := ss.AllAssignments[action.AssignmentId]
			if !ok {
				ke = kerror.Create("AssignmentNotFound", "assignment not found").With("assignmentId", action.AssignmentId)
				status = AS_Failed
				return
			}
			// update routing table
			assign.ShouldInRoutingTable = true
			ss.FlushWorkerState(ctx, workerFullId, ws, FS_WorkerState|FS_Routing, "addToRouting")
			status = AS_Completed
		})
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
			shardId := data.ShardId(action.ShardId)
			shardState, ok := ss.AllShards[shardId]
			if !ok {
				ke = kerror.Create("ShardNotFound", "shard not found").With("shardId", shardId)
				status = AS_Failed
				return
			}
			replicaState, ok := shardState.Replicas[action.ReplicaIdx]
			if !ok {
				ke = kerror.Create("ReplicaNotFound", "replica not found").With("shardId", shardId).With("replicaIdx", action.ReplicaIdx)
				status = AS_Failed
				return
			}
			_, ok = ss.AllAssignments[action.AssignmentId]
			if ok {
				ke = kerror.Create("AssignmentAlreadyExist", "assignment already exist").With("assignmentId", action.AssignmentId)
				status = AS_Failed
				return
			}
			workerState := ss.FindWorkerStateByWorkerFullId(workerFullId)
			if workerState == nil {
				ke = kerror.Create("DestWorkerNotFound", "worker not found").With("workerFullId", workerFullId)
				status = AS_Failed
				return
			}
			assign := NewAssignmentState(action.AssignmentId, shardId, action.ReplicaIdx, workerFullId)
			assign.TargetState = cougarjson.CAS_Ready
			assign.ShouldInPilot = true
			ss.AllAssignments[action.AssignmentId] = assign
			replicaState.Assignments[action.AssignmentId] = common.Unit{}
			workerState.Assignments[action.AssignmentId] = common.Unit{}
			ss.storeProvider.StoreShardState(shardId, shardState.ToJson())
			ss.FlushWorkerState(ctx, workerFullId, workerState, FS_WorkerState|FS_Pilot, "addShard")
			// wait on signal box
			signalBox = workerState.SignalBox
			status = AS_Wait
			klogging.Info(ctx).With("proposalId", am.moveState.ProposalId).With("worker", action.To).With("wallTime", kcommon.GetWallTimeMs()).With("status", status).Log("actionAddShard", "add shard to worker")
		})
		if status == AS_Failed {
			panic(ke)
		}
		action.ActionStage = smgjson.AS_Conducted
		am.ss.actionProvider.StoreActionNode(ctx, am.moveState.ProposalId, am.moveState.ToMoveStateJson("addShard"))
	}
	// step 2: wail until this assignment is stop (based on feedback from ephemeral node)
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
			assign, ok := ss.AllAssignments[action.AssignmentId]
			if !ok {
				reason = "assignment not found"
				return
			}
			if assign.CurrentConfirmedState == cougarjson.CAS_Ready {
				status = AS_Completed
				// apply to current snapshot
				ke = kcommon.TryCatchRun(ctx, func() {
					action.ApplyToSnapshot(ss.GetSnapshotCurrentForModify(), costfunc.AM_Strict)
				})
				if ke != nil {
					reason = "apply to snapshot failed:" + ke.FullString()
					status = AS_Failed
					return
				}
				return
			}
			reason = "assignment not in ready state"
		})

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
			replicaState, ok := shardState.Replicas[action.ReplicaIdx]
			if !ok {
				ke = kerror.Create("ReplicaNotFound", "replica not found").With("shardId", shardId).With("replicaIdx", action.ReplicaIdx)
				status = AS_Failed
				return
			}
			if _, ok := replicaState.Assignments[action.AssignmentId]; !ok {
				ke = kerror.Create("AssignmentNotFound", "assignment not found in replica").With("assignmentId", action.AssignmentId)
				status = AS_Failed
				return
			}
			assign, ok := ss.AllAssignments[action.AssignmentId]
			if !ok {
				ke = kerror.Create("AssignmentNotFound", "assignment not found").With("assignmentId", action.AssignmentId)
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
			ss.storeProvider.StoreShardState(shardId, shardState.ToJson())
			ss.FlushWorkerState(ctx, workerFullId, workerState, FS_Most, "dropShard")
			if workerState.IsOnline() {
				signalBox = workerState.SignalBox
			}
			status = AS_Wait
		})
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
					action.ApplyToSnapshot(ss.GetSnapshotCurrentForModify(), costfunc.AM_Relaxed)
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
			assign, ok := ss.AllAssignments[action.AssignmentId]
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
			if assign.CurrentConfirmedState == cougarjson.CAS_Dropped {
				status = AS_Completed
				applyToSnapshot()
				return
			}
			signalBox = workerState.SignalBox
		})
	}
	if status == AS_Failed {
		panic(ke)
	}
	action.ActionStage = smgjson.AS_Completed
	am.ss.actionProvider.StoreActionNode(ctx, am.moveState.ProposalId, am.moveState.ToMoveStateJson("dropShardDone"))
}

// ActionEvent implements IEvent
type ActionEvent struct {
	fn func(ss *ServiceState)
}

func NewActionEvent(fn func(ss *ServiceState)) *ActionEvent {
	return &ActionEvent{
		fn: fn,
	}
}

func (ae *ActionEvent) GetName() string {
	return "ActionEvent"
}

func (ae *ActionEvent) Process(ctx context.Context, ss *ServiceState) {
	ae.fn(ss)
}
