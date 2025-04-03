package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

type ActionMinion struct {
	ss        *ServiceState
	moveState *MoveState
}

func NewActionMinion(ss *ServiceState, moveState *MoveState) *ActionMinion {
	return &ActionMinion{
		ss:        ss,
		moveState: moveState,
	}
}

func (am *ActionMinion) Run(ctx context.Context, ss *ServiceState) {
	startMs := kcommon.GetWallTimeMs()
	ke := kcommon.TryCatchRun(ctx, func() {
		am.run(ctx)
	})
	elapseMs := kcommon.GetWallTimeMs() - startMs
	if ke != nil {
		klogging.Error(ctx).WithError(ke).With("signature", am.moveState.Signature).With("proposalId", am.moveState.ProposalId).With("elapsedMs", elapseMs).Log("ActionMinion", "Run")
	} else {
		klogging.Info(ctx).With("signature", am.moveState.Signature).With("proposalId", am.moveState.ProposalId).With("elapsedMs", elapseMs).Log("ActionMinion", "Run")
	}
	ss.actionProvider.StoreActionNode(ctx, am.moveState.ProposalId, nil)
}

func (am *ActionMinion) run(ctx context.Context) {
	klogging.Info(ctx).With("signature", am.moveState.Signature).With("proposalId", am.moveState.ProposalId).Log("ActionMinion", "Run")
	for am.moveState.CurrentAction < len(am.moveState.Actions) {
		action := am.moveState.Actions[am.moveState.CurrentAction]
		klogging.Info(ctx).With("action", action).Log("ActionMinion", "Run")
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
		am.moveState.ActionConducted = true
	}
}

// in case anything goes wrong, we will panic
func (am *ActionMinion) actionRemoveFromRouting(ctx context.Context, stepIdx int) {
	action := am.moveState.Actions[stepIdx]
	if !am.moveState.ActionConducted {
		workerFullId := action.From
		// step 1: remove from routing table
		succ := true
		var ke *kerror.Kerror
		am.ss.PostEvent(NewActionEvent(func(ss *ServiceState) {
			ws, ok := am.ss.AllWorkers[workerFullId]
			if !ok {
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
			assign.ShouldInRoutingTable = false
			ss.FlushWorkerState(ctx, workerFullId, ws, "removeFromRouting")
		}))
		if !succ {
			panic(ke)
		}
		am.moveState.ActionConducted = true
		am.ss.actionProvider.StoreActionNode(ctx, am.moveState.ProposalId, am.moveState.ToMoveStateJson())
	}

	// step 2: sleep
	if action.SleepMs > 0 {
		kcommon.SleepMs(ctx, action.SleepMs)
	}
}

func (am *ActionMinion) actionAddToRouting(ctx context.Context, stepIdx int) {
	action := am.moveState.Actions[stepIdx]
	if !am.moveState.ActionConducted {
		workerFullId := action.To
		// step 1: add to routing table
		succ := true
		var ke *kerror.Kerror
		am.ss.PostEvent(NewActionEvent(func(ss *ServiceState) {
			ws, ok := am.ss.AllWorkers[workerFullId]
			if !ok {
				succ = false
				ke = kerror.Create("DestWorkerNotFound", "worker not found").With("workerFullId", workerFullId)
				return
			}
			assign, ok := ss.AllAssignments[action.AssignmentId]
			if !ok {
				succ = false
				ke = kerror.Create("AssignmentNotFound", "assignment not found").With("assignmentId", action.AssignmentId)
				return
			}
			assign.ShouldInRoutingTable = true
			ss.FlushWorkerState(ctx, workerFullId, ws, "addToRouting")
		}))
		if !succ {
			panic(ke)
		}
		am.moveState.ActionConducted = true
		am.ss.actionProvider.StoreActionNode(ctx, am.moveState.ProposalId, am.moveState.ToMoveStateJson())
	}
}

func (am *ActionMinion) actionAddShard(ctx context.Context, stepIdx int) {
	var signalBox *SignalBox
	action := am.moveState.Actions[stepIdx]
	// step 1: create shard
	if !am.moveState.ActionConducted {
		var ke *kerror.Kerror
		workerFullId := action.To
		am.ss.PostActionAndWait(func(ss *ServiceState) {
			shardId := data.ShardId(action.ShardId)
			shardState, ok := ss.AllShards[shardId]
			if !ok {
				ke = kerror.Create("ShardNotFound", "shard not found").With("shardId", shardId)
				return
			}
			replicaState, ok := shardState.Replicas[action.ReplicaIdx]
			if !ok {
				ke = kerror.Create("ReplicaNotFound", "replica not found").With("shardId", shardId).With("replicaIdx", action.ReplicaIdx)
				return
			}
			_, ok = ss.AllAssignments[action.AssignmentId]
			if ok {
				ke = kerror.Create("AssignmentAlreadyExist", "assignment already exist").With("assignmentId", action.AssignmentId)
				return
			}
			workerState, ok := ss.AllWorkers[workerFullId]
			if !ok {
				ke = kerror.Create("DestWorkerNotFound", "worker not found").With("workerFullId", workerFullId)
				return
			}
			assign := NewAssignmentState(action.AssignmentId, shardId, action.ReplicaIdx, workerFullId)
			assign.TargetState = cougarjson.CAS_Ready
			assign.ShouldInPilot = true
			ss.AllAssignments[action.AssignmentId] = assign
			replicaState.Assignments[action.AssignmentId] = common.Unit{}
			workerState.Assignments[action.AssignmentId] = common.Unit{}
			ss.storeProvider.StoreShardState(shardId, shardState.ToJson())
			ss.FlushWorkerState(ctx, workerFullId, workerState, "addShard")
			signalBox = workerState.SignalBox
		})
		if ke != nil {
			panic(ke)
		}
		am.moveState.ActionConducted = true
		am.ss.actionProvider.StoreActionNode(ctx, am.moveState.ProposalId, am.moveState.ToMoveStateJson())
	}
	// step 2: wail until this assignment is stop (based on feedback from ephemeral node)
	klogging.Info(ctx).With("proposalId", am.moveState.ProposalId).With("worker", action.To).With("wallTime", kcommon.GetWallTimeMs()).Log("actionAddShard", "wait for assignment to be ready")
	stop := false
	for !stop {
		loopId := kcommon.RandomString(ctx, 8)
		if signalBox != nil {
			select {
			case <-ctx.Done():
				panic(kerror.Create("ContextCanceled", "context canceled"))
			case <-signalBox.NotifyCh:
				klogging.Info(ctx).With("proposalId", am.moveState.ProposalId).With("worker", action.To).With("wallTime", kcommon.GetWallTimeMs()).With("notifyReason", signalBox.NotifyReason).With("notifyId", signalBox.NofityId).With("loopId", loopId).Log("actionAddShard", "wake up")
			}
		}
		requirementMeet := false
		var ke *kerror.Kerror
		var reason string
		am.ss.PostActionAndWait(func(ss *ServiceState) {
			workerState, ok := ss.AllWorkers[action.To]
			if !ok {
				ke = kerror.Create("DestWorkerNotFound", "worker not found").With("workerFullId", action.To)
				return
			}
			if !workerState.IsGoodMoveTarget() {
				ke = kerror.Create("DestWorkerNotGood", "worker not suitable as target").With("workerFullId", action.To).With("state", workerState.State)
				return
			}
			signalBox = workerState.SignalBox
			assign, ok := ss.AllAssignments[action.AssignmentId]
			if !ok {
				reason = "assignment not found"
				return
			}
			if assign.CurrentConfirmedState == cougarjson.CAS_Ready {
				requirementMeet = true
				return
			}
			reason = "assignment not in ready state"
		})

		if ke != nil {
			panic(ke)
		}
		if requirementMeet {
			// success
			stop = true
		}
		klogging.Info(ctx).With("proposalId", am.moveState.ProposalId).With("worker", action.To).With("wallTime", kcommon.GetWallTimeMs()).With("loopId", loopId).With("reason", reason).With("reqMeet", requirementMeet).With("stop", stop).Log("actionAddShard", "loop end")
	}
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
	var signalBox *SignalBox
	action := am.moveState.Actions[stepIdx]
	// step 1: drop shard
	if !am.moveState.ActionConducted {
		succ := true
		var ke *kerror.Kerror
		workerFullId := action.From
		am.ss.PostActionAndWait(func(ss *ServiceState) {
			shardId := data.ShardId(action.ShardId)
			shardState, ok := ss.AllShards[shardId]
			if !ok {
				succ = false
				ke = kerror.Create("ShardNotFound", "shard not found").With("shardId", shardId)
				return
			}
			replicaState, ok := shardState.Replicas[action.ReplicaIdx]
			if !ok {
				succ = false
				ke = kerror.Create("ReplicaNotFound", "replica not found").With("shardId", shardId).With("replicaIdx", action.ReplicaIdx)
				return
			}
			if _, ok := replicaState.Assignments[action.AssignmentId]; !ok {
				succ = false
				ke = kerror.Create("AssignmentNotFound", "assignment not found in replica").With("assignmentId", action.AssignmentId)
				return
			}
			assign, ok := ss.AllAssignments[action.AssignmentId]
			if !ok {
				succ = false
				ke = kerror.Create("AssignmentNotFound", "assignment not found").With("assignmentId", action.AssignmentId)
				return
			}
			workerState, ok := ss.AllWorkers[workerFullId]
			if !ok {
				succ = false
				ke = kerror.Create("SrcWorkerNotFound", "worker not found").With("workerFullId", workerFullId)
				return
			}
			assign.ShouldInPilot = false
			ss.storeProvider.StoreShardState(shardId, shardState.ToJson())
			ss.FlushWorkerState(ctx, workerFullId, workerState, "dropShard")
			signalBox = workerState.SignalBox
		})
		if !succ {
			panic(ke)
		}
		am.moveState.ActionConducted = true
		am.ss.actionProvider.StoreActionNode(ctx, am.moveState.ProposalId, am.moveState.ToMoveStateJson())
	}
	// step 2: wail until drop assignment is completed (based on feedback from ephemeral node)
	completed := false
	for !completed {
		if signalBox != nil {
			select {
			case <-ctx.Done():
				panic(kerror.Create("ContextCanceled", "context canceled"))
			case <-signalBox.NotifyCh:
				klogging.Info(ctx).With("proposalId", am.moveState.ProposalId).With("worker", action.From).With("wallTime", kcommon.GetWallTimeMs()).With("notifyReason", signalBox.NotifyReason).With("notifyId", signalBox.NofityId).Log("actionDropShard", "wake up")
			}
		}
		succ := true
		var ke *kerror.Kerror
		am.ss.PostActionAndWait(func(ss *ServiceState) {
			workerState, ok := ss.AllWorkers[action.From]
			if !ok {
				succ = false
				ke = kerror.Create("SrcWorkerNotFound", "worker not found").With("workerFullId", action.From)
				return
			}
			assign, ok := ss.AllAssignments[action.AssignmentId]
			if !ok {
				succ = false
				ke = kerror.Create("AssignmentNotFound", "assignment not found").With("assignmentId", action.AssignmentId)
				return
			}
			if assign.CurrentConfirmedState == cougarjson.CAS_Dropped {
				completed = true
				return
			}
			signalBox = workerState.SignalBox
		})
		if !succ {
			panic(ke)
		}
	}
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
