package costfunc

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
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

	Move    Move
	OnClose func(reason common.EnqueueResult) // will get called when proposal is closed
}

func NewProposal(ctx context.Context, solverType string, gain Gain, basedOn SnapshotId) *Proposal {
	return &Proposal{
		ProposalId:  data.ProposalId(kcommon.RandomString(ctx, 8)),
		SolverType:  solverType,
		StartTimeMs: kcommon.GetWallTimeMs(),
	}
}

type Move interface {
	GetSignature() string
	Apply(snapshot *Snapshot)
	GetActions(cfg config.ShardConfig) []*smgjson.ActionJson
}

func (proposal *Proposal) GetSignature() string {
	return proposal.Move.GetSignature()
}

// SimpleMove implements Move
type SimpleMove struct {
	Replica          data.ReplicaFullId
	SrcAssignmentId  data.AssignmentId
	DestAssignmentId data.AssignmentId
	Src              data.WorkerFullId
	Dst              data.WorkerFullId
}

func (move *SimpleMove) GetSignature() string {
	return move.Replica.String() + "/" + move.Src.String() + "/" + move.Dst.String()
}

func (move *SimpleMove) Apply(snapshot *Snapshot) {
	snapshot.Unassign(move.Src, move.Replica.ShardId, move.Replica.ReplicaIdx, move.SrcAssignmentId)
	snapshot.Assign(move.Replica.ShardId, move.Replica.ReplicaIdx, move.DestAssignmentId, move.Dst)
}

func (move *SimpleMove) GetActions(cfg config.ShardConfig) []*smgjson.ActionJson {
	var list []*smgjson.ActionJson
	if cfg.MovePolicy == smgjson.MP_KillBeforeStart {
		// step 1: remove src from routing
		list = append(list, &smgjson.ActionJson{
			ActionType:           smgjson.AT_RemoveFromRoutingAndSleep,
			ShardId:              move.Replica.ShardId,
			ReplicaIdx:           move.Replica.ReplicaIdx,
			AssignmentId:         move.SrcAssignmentId,
			From:                 move.Src.String(),
			RemoveSrcFromRouting: 1,
			SleepMs:              1000, // sleep 1s
		})
		// step 2: drop
		list = append(list, &smgjson.ActionJson{
			ActionType:   smgjson.AT_DropShard,
			ShardId:      move.Replica.ShardId,
			ReplicaIdx:   move.Replica.ReplicaIdx,
			AssignmentId: move.SrcAssignmentId,
			From:         move.Src.String(),
		})
		// step 3: add shard
		list = append(list, &smgjson.ActionJson{
			ActionType:   smgjson.AT_AddShard,
			ShardId:      move.Replica.ShardId,
			ReplicaIdx:   move.Replica.ReplicaIdx,
			AssignmentId: move.DestAssignmentId,
			To:           move.Dst.String(),
		})
		// step 4: add dest to routing
		list = append(list, &smgjson.ActionJson{
			ActionType:       smgjson.AT_AddToRouting,
			ShardId:          move.Replica.ShardId,
			ReplicaIdx:       move.Replica.ReplicaIdx,
			AssignmentId:     move.DestAssignmentId,
			To:               move.Dst.String(),
			AddDestToRouting: 1,
		})
		return list
	} else if cfg.MovePolicy == smgjson.MP_StartBeforeKill {
		// step 1: add shard
		list = append(list, &smgjson.ActionJson{
			ActionType:   smgjson.AT_AddShard,
			ShardId:      move.Replica.ShardId,
			ReplicaIdx:   move.Replica.ReplicaIdx,
			AssignmentId: move.DestAssignmentId,
			To:           move.Dst.String(),
		})
		// step 2: add dest to routing
		list = append(list, &smgjson.ActionJson{
			ActionType:       smgjson.AT_AddToRouting,
			ShardId:          move.Replica.ShardId,
			ReplicaIdx:       move.Replica.ReplicaIdx,
			AssignmentId:     move.DestAssignmentId,
			To:               move.Dst.String(),
			AddDestToRouting: 1,
		})
		// step 3: remove src from routing
		list = append(list, &smgjson.ActionJson{
			ActionType:           smgjson.AT_RemoveFromRoutingAndSleep,
			ShardId:              move.Replica.ShardId,
			ReplicaIdx:           move.Replica.ReplicaIdx,
			AssignmentId:         move.SrcAssignmentId,
			From:                 move.Src.String(),
			RemoveSrcFromRouting: 1,
			SleepMs:              1000, // sleep 1s
		})
		// step 4: drop
		list = append(list, &smgjson.ActionJson{
			ActionType:   smgjson.AT_DropShard,
			ShardId:      move.Replica.ShardId,
			ReplicaIdx:   move.Replica.ReplicaIdx,
			AssignmentId: move.SrcAssignmentId,
			From:         move.Src.String(),
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

func (move *AssignMove) GetSignature() string {
	return move.Replica.String() + "/" + move.Worker.String()
}

func (move *AssignMove) Apply(snapshot *Snapshot) {
	snapshot.Assign(move.Replica.ShardId, move.Replica.ReplicaIdx, move.AssignmentId, move.Worker)
}

func (move *AssignMove) GetActions(cfg config.ShardConfig) []*smgjson.ActionJson {
	var list []*smgjson.ActionJson
	list = append(list, &smgjson.ActionJson{
		ActionType:   smgjson.AT_AddShard,
		ShardId:      move.Replica.ShardId,
		ReplicaIdx:   move.Replica.ReplicaIdx,
		AssignmentId: move.AssignmentId,
		To:           move.Worker.String(),
	})
	return list
}

// UnassignMove implements Move
type UnassignMove struct {
	Worker       data.WorkerFullId
	Replica      data.ReplicaFullId
	AssignmentId data.AssignmentId
}

func (move *UnassignMove) GetSignature() string {
	return move.Worker.String() + "/" + move.Replica.String()
}

func (move *UnassignMove) Apply(snapshot *Snapshot) {
	snapshot.Unassign(move.Worker, move.Replica.ShardId, move.Replica.ReplicaIdx, move.AssignmentId)
}

func (move *UnassignMove) GetActions(cfg config.ShardConfig) []*smgjson.ActionJson {
	var list []*smgjson.ActionJson
	list = append(list, &smgjson.ActionJson{
		ActionType:   smgjson.AT_DropShard,
		ShardId:      move.Replica.ShardId,
		ReplicaIdx:   move.Replica.ReplicaIdx,
		AssignmentId: move.AssignmentId,
		From:         move.Worker.String(),
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
