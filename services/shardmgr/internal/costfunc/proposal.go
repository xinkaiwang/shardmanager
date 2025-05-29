package costfunc

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
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
	From                 data.WorkerFullId
	SrcReplicaIdx        data.ReplicaIdx
	SrcAssignmentId      data.AssignmentId
	RemoveSrcFromRouting bool
	To                   data.WorkerFullId
	DestReplicaIdx       data.ReplicaIdx
	DestAssignmentId     data.AssignmentId
	AddDestToRouting     bool
	SleepMs              int
	ActionStage          smgjson.ActionStage
	DeleteReplica        bool
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
		From:                 action.From.String(),
		SrcReplicaIdx:        action.SrcReplicaIdx,
		SrcAssignmentId:      action.SrcAssignmentId,
		RemoveSrcFromRouting: 0,
		To:                   action.To.String(),
		DestReplicaIdx:       action.DestReplicaIdx,
		DestAssignmentId:     action.DestAssignmentId,
		AddDestToRouting:     0,
		SleepMs:              action.SleepMs,
		Stage:                action.ActionStage,
	}
	return actionJson
}

func (action *Action) String() string {
	return string(action.ActionType)
}

func ActionFromJson(actionJson *smgjson.ActionJson) *Action {
	action := &Action{
		ActionType:           actionJson.ActionType,
		ShardId:              actionJson.ShardId,
		From:                 data.WorkerFullIdParseFromString(actionJson.From),
		SrcReplicaIdx:        actionJson.SrcReplicaIdx,
		SrcAssignmentId:      actionJson.SrcAssignmentId,
		RemoveSrcFromRouting: common.BoolFromInt8(actionJson.RemoveSrcFromRouting),
		To:                   data.WorkerFullIdParseFromString(actionJson.To),
		DestReplicaIdx:       actionJson.DestReplicaIdx,
		DestAssignmentId:     actionJson.DestAssignmentId,
		AddDestToRouting:     common.BoolFromInt8(actionJson.AddDestToRouting),
		SleepMs:              actionJson.SleepMs,
		ActionStage:          actionJson.Stage,
	}
	return action
}

func (action *Action) ApplyToSnapshot(snapshot *Snapshot, mode ApplyMode) *Snapshot {
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
	return snapshot
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
	replicaState, ok := shardState.Replicas[action.DestReplicaIdx]
	if !ok {
		// replica not found, create a new one
		replicaState = NewReplicaSnap(action.ShardId, action.DestReplicaIdx)
	}
	// when reach here, the destWorker/shard/replica confirmed exist
	// copy on write (workerSnap)
	newWorkerSnap := workerState.Clone()
	newWorkerSnap.Assignments[action.ShardId] = action.DestAssignmentId
	snapshot.AllWorkers.Set(action.To, newWorkerSnap)
	// copy on write (shardSnap)
	newReplicaSnap := replicaState.Clone()
	newReplicaSnap.Assignments[action.DestAssignmentId] = common.Unit{}
	newShardSnap := shardState.Clone()
	newShardSnap.Replicas[action.DestReplicaIdx] = newReplicaSnap
	snapshot.AllShards.Set(action.ShardId, newShardSnap)

	snapshot.AllAssignments.Set(action.DestAssignmentId, &AssignmentSnap{
		ShardId:      action.ShardId,
		ReplicaIdx:   action.DestReplicaIdx,
		AssignmentId: action.DestAssignmentId,
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
	if assignmentId != action.SrcAssignmentId {
		if mode == AM_Strict {
			ke := kerror.Create("AssignmentIdMismatch", "assignment id mismatch").With("shardId", action.ShardId).With("assignmentId", assignmentId).With("expectedAssignmentId", action.SrcAssignmentId)
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
	replicaState, ok := shardState.Replicas[action.SrcReplicaIdx]
	if !ok {
		if mode == AM_Strict {
			ke := kerror.Create("ReplicaNotFound", "replica not found").With("shardId", action.ShardId).With("replicaIdx", action.SrcReplicaIdx)
			panic(ke)
		} else if mode == AM_Relaxed {
			return
		} else {
			klogging.Fatal(context.Background()).With("mode", mode).Log("UnknownApplyMode", "")
		}
	}
	// when reach here, the srcWorker/assignment/shard/replica confirmed exist
	// copy-on-write (workerSnap)
	newWorkerSnap := workerState.Clone()
	delete(newWorkerSnap.Assignments, action.ShardId)
	snapshot.AllWorkers.Set(action.From, newWorkerSnap)
	// copy-on-write (shardSnap)
	newReplicaSnap := replicaState.Clone()
	newShardSnap := shardState.Clone()
	if action.DeleteReplica {
		newReplicaSnap.LameDuck = true
	}
	newShardSnap.Replicas[action.SrcReplicaIdx] = newReplicaSnap
	snapshot.AllShards.Set(action.ShardId, newShardSnap)
	delete(newReplicaSnap.Assignments, action.SrcAssignmentId)
	snapshot.AllAssignments.Delete(action.SrcAssignmentId)
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
