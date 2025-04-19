package costfunc

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

// Proposal implements OrderedListItem
type Proposal struct {
	ProposalId  data.ProposalId
	SolverType  string
	Gain        Gain
	BasedOn     SnapshotId
	StartTimeMs int64 // epoch time in ms

	Move         Move
	ProposalSize int                               // size of the proposal: foe example, 1 for simple move, 2 for swap move
	Signature    string                            // signature of the move (redundant info, but useful for debugging)
	OnClose      func(reason common.EnqueueResult) // will get called when proposal is closed
}

func NewProposal(ctx context.Context, solverType string, gain Gain, basedOn SnapshotId) *Proposal {
	proposal := &Proposal{
		ProposalId:   data.ProposalId(kcommon.RandomString(ctx, 8)),
		SolverType:   solverType,
		Gain:         gain,
		BasedOn:      basedOn,
		StartTimeMs:  kcommon.GetWallTimeMs(),
		ProposalSize: 1,
	}
	return proposal
}

type Action struct {
	ActionType           smgjson.ActionType
	ShardId              data.ShardId
	ReplicaIdx           data.ReplicaIdx
	AssignmentId         data.AssignmentId
	From                 data.WorkerFullId
	To                   data.WorkerFullId
	RemoveSrcFromRouting bool
	AddDestToRouting     bool
	SleepMs              int
	ActionStage          smgjson.ActionStage
}

func NewAction(actionType smgjson.ActionType) *Action {
	return &Action{
		ActionType:  actionType,
		ActionStage: smgjson.AS_NotStarted,
	}
}

func (action *Action) ToJson() *smgjson.ActionJson {
	actionJson := &smgjson.ActionJson{
		ActionType:           action.ActionType,
		ShardId:              action.ShardId,
		ReplicaIdx:           action.ReplicaIdx,
		AssignmentId:         action.AssignmentId,
		From:                 action.From.String(),
		To:                   action.To.String(),
		RemoveSrcFromRouting: 0,
		AddDestToRouting:     0,
		SleepMs:              action.SleepMs,
		Stage:                action.ActionStage,
	}
	return actionJson
}

func ActionFromJson(actionJson *smgjson.ActionJson) *Action {
	action := &Action{
		ActionType:           actionJson.ActionType,
		ShardId:              actionJson.ShardId,
		ReplicaIdx:           actionJson.ReplicaIdx,
		AssignmentId:         actionJson.AssignmentId,
		From:                 data.WorkerFullIdParseFromString(actionJson.From),
		To:                   data.WorkerFullIdParseFromString(actionJson.To),
		RemoveSrcFromRouting: common.BoolFromInt8(actionJson.RemoveSrcFromRouting),
		AddDestToRouting:     common.BoolFromInt8(actionJson.AddDestToRouting),
		SleepMs:              actionJson.SleepMs,
		ActionStage:          actionJson.Stage,
	}
	return action
}

func (action *Action) ApplyToSnapshot(snapshot *Snapshot, mode ApplyMode) {
	switch action.ActionType {
	case smgjson.AT_AddShard:
		action.applyAddShard(snapshot, mode)
	case smgjson.AT_DropShard:
		action.applyDropShard(snapshot, mode)
	case smgjson.AT_RemoveFromRoutingAndSleep, smgjson.AT_AddToRouting: // nothing to do
		break
	default:
		klogging.Fatal(context.Background()).With("actionType", action.ActionType).Log("UnknownActionType", "")
	}
}

func (action *Action) applyAddShard(snapshot *Snapshot, mode ApplyMode) {
	workerState, ok := snapshot.AllWorkers.Get(action.To)
	if !ok {
		if mode == AM_Strict {
			ke := kerror.Create("WorkerNotFound", "worker not found").With("workerId", action.To)
			panic(ke)
		} else if mode == AM_Relaxed {
			return
		} else {
			klogging.Fatal(context.Background()).With("mode", mode).Log("UnknownApplyMode", "")
		}
	}
	shardState, ok := snapshot.AllShards.Get(action.ShardId)
	if !ok {
		if mode == AM_Strict {
			ke := kerror.Create("ShardNotFound", "shard not found").With("shardId", action.ShardId)
			panic(ke)
		} else if mode == AM_Relaxed {
			return
		} else {
			klogging.Fatal(context.Background()).With("mode", mode).Log("UnknownApplyMode", "")
		}
	}
	replicaState, ok := shardState.Replicas[action.ReplicaIdx]
	if !ok {
		if mode == AM_Strict {
			ke := kerror.Create("ReplicaNotFound", "replica not found").With("shardId", action.ShardId).With("replicaIdx", action.ReplicaIdx)
			panic(ke)
		} else if mode == AM_Relaxed {
			return
		} else {
			klogging.Fatal(context.Background()).With("mode", mode).Log("UnknownApplyMode", "")
		}
	}
	// when reach here, the destWorker/shard/replica confirmed exist
	workerState.Assignments[action.ShardId] = action.AssignmentId
	replicaState.Assignments[action.AssignmentId] = common.Unit{}
	snapshot.AllAssignments.Set(action.AssignmentId, &AssignmentSnap{
		ShardId:      action.ShardId,
		ReplicaIdx:   action.ReplicaIdx,
		AssignmentId: action.AssignmentId,
		WorkerFullId: action.To,
	})
}

func (action *Action) applyDropShard(snapshot *Snapshot, mode ApplyMode) {
	workerState, ok := snapshot.AllWorkers.Get(action.From)
	if !ok {
		if mode == AM_Strict {
			ke := kerror.Create("WorkerNotFound", "worker not found").With("workerId", action.To)
			panic(ke)
		} else if mode == AM_Relaxed {
			return
		} else {
			klogging.Fatal(context.Background()).With("mode", mode).Log("UnknownApplyMode", "")
		}
	}
	assignmentId, ok := workerState.Assignments[action.ShardId]
	if !ok {
		if mode == AM_Strict {
			ke := kerror.Create("AssignmentNotFound", "assignment not found").With("shardId", action.ShardId)
			panic(ke)
		} else if mode == AM_Relaxed {
			return
		} else {
			klogging.Fatal(context.Background()).With("mode", mode).Log("UnknownApplyMode", "")
		}
	}
	if assignmentId != action.AssignmentId {
		if mode == AM_Strict {
			ke := kerror.Create("AssignmentIdMismatch", "assignment id mismatch").With("shardId", action.ShardId).With("assignmentId", assignmentId).With("expectedAssignmentId", action.AssignmentId)
			panic(ke)
		} else if mode == AM_Relaxed {
			return
		} else {
			klogging.Fatal(context.Background()).With("mode", mode).Log("UnknownApplyMode", "")
		}
	}
	shardState, ok := snapshot.AllShards.Get(action.ShardId)
	if !ok {
		if mode == AM_Strict {
			ke := kerror.Create("ShardNotFound", "shard not found").With("shardId", action.ShardId)
			panic(ke)
		} else if mode == AM_Relaxed {
			return
		} else {
			klogging.Fatal(context.Background()).With("mode", mode).Log("UnknownApplyMode", "")
		}
	}
	replicaState, ok := shardState.Replicas[action.ReplicaIdx]
	if !ok {
		if mode == AM_Strict {
			ke := kerror.Create("ReplicaNotFound", "replica not found").With("shardId", action.ShardId).With("replicaIdx", action.ReplicaIdx)
			panic(ke)
		} else if mode == AM_Relaxed {
			return
		} else {
			klogging.Fatal(context.Background()).With("mode", mode).Log("UnknownApplyMode", "")
		}
	}
	// when reach here, the srcWorker/assignment/shard/replica confirmed exist
	delete(workerState.Assignments, action.ShardId)
	delete(replicaState.Assignments, action.AssignmentId)
	snapshot.AllAssignments.Delete(action.AssignmentId)
}

type Move interface {
	GetSignature() string
	Apply(snapshot *Snapshot, mode ApplyMode)
	GetActions(cfg config.ShardConfig) []*Action
}

func (proposal *Proposal) GetSignature() string {
	return proposal.Move.GetSignature()
}

func (proposal *Proposal) GetSize() int {
	return proposal.ProposalSize
}

func (proposal *Proposal) GetEfficiency() Gain {
	return Gain{
		HardScore: proposal.Gain.HardScore,
		SoftScore: proposal.Gain.SoftScore / float64(proposal.ProposalSize),
	}
}

// SimpleMove implements Move
type SimpleMove struct {
	Replica          data.ReplicaFullId
	SrcAssignmentId  data.AssignmentId
	DestAssignmentId data.AssignmentId
	Src              data.WorkerFullId
	Dst              data.WorkerFullId
}

func NewSimpleMove(replica data.ReplicaFullId, srcAssignmentId data.AssignmentId, destAssignmentId data.AssignmentId, src data.WorkerFullId, dst data.WorkerFullId) *SimpleMove {
	return &SimpleMove{
		Replica:          replica,
		SrcAssignmentId:  srcAssignmentId,
		DestAssignmentId: destAssignmentId,
		Src:              src,
		Dst:              dst,
	}
}

func (move *SimpleMove) GetSignature() string {
	return move.Replica.String() + "/" + move.Src.String() + "/" + move.Dst.String()
}

func (move *SimpleMove) Apply(snapshot *Snapshot, mode ApplyMode) {
	snapshot.Unassign(move.Src, move.Replica.ShardId, move.Replica.ReplicaIdx, move.SrcAssignmentId, mode)
	snapshot.Assign(move.Replica.ShardId, move.Replica.ReplicaIdx, move.DestAssignmentId, move.Dst, mode)
}

func (move *SimpleMove) GetActions(cfg config.ShardConfig) []*Action {
	var list []*Action
	if cfg.MovePolicy == smgjson.MP_KillBeforeStart {
		// step 1: remove src from routing
		list = append(list, &Action{
			ActionType:           smgjson.AT_RemoveFromRoutingAndSleep,
			ShardId:              move.Replica.ShardId,
			ReplicaIdx:           move.Replica.ReplicaIdx,
			AssignmentId:         move.SrcAssignmentId,
			From:                 move.Src,
			RemoveSrcFromRouting: true,
			SleepMs:              1000, // sleep 1s
		})
		// step 2: drop
		list = append(list, &Action{
			ActionType:   smgjson.AT_DropShard,
			ShardId:      move.Replica.ShardId,
			ReplicaIdx:   move.Replica.ReplicaIdx,
			AssignmentId: move.SrcAssignmentId,
			From:         move.Src,
		})
		// step 3: add shard
		list = append(list, &Action{
			ActionType:   smgjson.AT_AddShard,
			ShardId:      move.Replica.ShardId,
			ReplicaIdx:   move.Replica.ReplicaIdx,
			AssignmentId: move.DestAssignmentId,
			To:           move.Dst,
		})
		// step 4: add dest to routing
		list = append(list, &Action{
			ActionType:       smgjson.AT_AddToRouting,
			ShardId:          move.Replica.ShardId,
			ReplicaIdx:       move.Replica.ReplicaIdx,
			AssignmentId:     move.DestAssignmentId,
			To:               move.Dst,
			AddDestToRouting: true,
		})
		return list
	} else if cfg.MovePolicy == smgjson.MP_StartBeforeKill {
		// step 1: add shard
		list = append(list, &Action{
			ActionType:   smgjson.AT_AddShard,
			ShardId:      move.Replica.ShardId,
			ReplicaIdx:   move.Replica.ReplicaIdx,
			AssignmentId: move.DestAssignmentId,
			To:           move.Dst,
		})
		// step 2: add dest to routing
		list = append(list, &Action{
			ActionType:       smgjson.AT_AddToRouting,
			ShardId:          move.Replica.ShardId,
			ReplicaIdx:       move.Replica.ReplicaIdx,
			AssignmentId:     move.DestAssignmentId,
			To:               move.Dst,
			AddDestToRouting: true,
		})
		// step 3: remove src from routing
		list = append(list, &Action{
			ActionType:           smgjson.AT_RemoveFromRoutingAndSleep,
			ShardId:              move.Replica.ShardId,
			ReplicaIdx:           move.Replica.ReplicaIdx,
			AssignmentId:         move.SrcAssignmentId,
			From:                 move.Src,
			RemoveSrcFromRouting: true,
			SleepMs:              1000, // sleep 1s
		})
		// step 4: drop
		list = append(list, &Action{
			ActionType:   smgjson.AT_DropShard,
			ShardId:      move.Replica.ShardId,
			ReplicaIdx:   move.Replica.ReplicaIdx,
			AssignmentId: move.SrcAssignmentId,
			From:         move.Src,
		})
		return list
	} else {
		klogging.Fatal(context.Background()).With("policy", cfg.MovePolicy).Log("UnknownMovePolicy", "")
		return list
	}
}

// AssignMove implements Move
type AssignMove struct {
	Replica      data.ReplicaFullId
	AssignmentId data.AssignmentId
	Worker       data.WorkerFullId
}

func NewAssignMove(replica data.ReplicaFullId, assignmentId data.AssignmentId, worker data.WorkerFullId) *AssignMove {
	return &AssignMove{
		Replica:      replica,
		AssignmentId: assignmentId,
		Worker:       worker,
	}
}

func (move *AssignMove) GetSignature() string {
	return move.Replica.String() + "/" + move.Worker.String()
}

func (move *AssignMove) Apply(snapshot *Snapshot, mode ApplyMode) {
	snapshot.Assign(move.Replica.ShardId, move.Replica.ReplicaIdx, move.AssignmentId, move.Worker, mode)
}

func (move *AssignMove) GetActions(cfg config.ShardConfig) []*Action {
	var list []*Action
	// step 1: add shard
	list = append(list, &Action{
		ActionType:   smgjson.AT_AddShard,
		ShardId:      move.Replica.ShardId,
		ReplicaIdx:   move.Replica.ReplicaIdx,
		AssignmentId: move.AssignmentId,
		To:           move.Worker,
	})
	// step 2: add dest to routing
	list = append(list, &Action{
		ActionType:       smgjson.AT_AddToRouting,
		ShardId:          move.Replica.ShardId,
		ReplicaIdx:       move.Replica.ReplicaIdx,
		AssignmentId:     move.AssignmentId,
		To:               move.Worker,
		AddDestToRouting: true,
	})
	return list
}

// UnassignMove implements Move
type UnassignMove struct {
	Worker       data.WorkerFullId
	Replica      data.ReplicaFullId
	AssignmentId data.AssignmentId
}

func NewUnassignMove(worker data.WorkerFullId, replica data.ReplicaFullId, assignmentId data.AssignmentId) *UnassignMove {
	return &UnassignMove{
		Worker:       worker,
		Replica:      replica,
		AssignmentId: assignmentId,
	}
}

func (move *UnassignMove) GetSignature() string {
	return move.Worker.String() + "/" + move.Replica.String()
}

func (move *UnassignMove) Apply(snapshot *Snapshot, mode ApplyMode) {
	snapshot.Unassign(move.Worker, move.Replica.ShardId, move.Replica.ReplicaIdx, move.AssignmentId, mode)
}

func (move *UnassignMove) GetActions(cfg config.ShardConfig) []*Action {
	var list []*Action
	// step 1: remove from routing
	list = append(list, &Action{
		ActionType:           smgjson.AT_RemoveFromRoutingAndSleep,
		ShardId:              move.Replica.ShardId,
		ReplicaIdx:           move.Replica.ReplicaIdx,
		AssignmentId:         move.AssignmentId,
		From:                 move.Worker,
		RemoveSrcFromRouting: true,
		SleepMs:              1000, // sleep 1s
	})
	// step 2: drop
	list = append(list, &Action{
		ActionType:   smgjson.AT_DropShard,
		ShardId:      move.Replica.ShardId,
		ReplicaIdx:   move.Replica.ReplicaIdx,
		AssignmentId: move.AssignmentId,
		From:         move.Worker,
	})
	return list
}

// SwapMove implements Move
type SwapMove struct {
	Replica1 data.ReplicaFullId
	Replica2 data.ReplicaFullId
	Src      data.WorkerFullId
	Dst      data.WorkerFullId
}

func (move *SwapMove) GetSignature() string {
	return move.Replica1.String() + "/" + move.Replica2.String() + "/" + move.Src.String() + "/" + move.Dst.String()
}

// ReplaceMove implements Move
type ReplaceMove struct {
	ReplicaOut data.ReplicaFullId
	ReplicaIn  data.ReplicaFullId
	Worker     data.WorkerFullId
}

func (move *ReplaceMove) GetSignature() string {
	return move.ReplicaOut.String() + "/" + move.ReplicaIn.String() + "/" + move.Worker.String()
}

// implemnts OrderedListItem
func (prop *Proposal) IsBetterThan(other common.OrderedListItem) bool {
	otherProp := other.(*Proposal)
	return prop.Gain.IsGreaterThan(otherProp.Gain)
}

// implemnts OrderedListItem
func (prop *Proposal) Dropped(ctx context.Context, reason common.EnqueueResult) {
	klogging.Debug(ctx).With("proposalId", prop.ProposalId).With("solver", prop.SolverType).With("signature", prop.GetSignature()).Log("ProposalClosed", string(reason))
}
