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

	Move         Move
	ProposalSize int                               // size of the proposal: foe example, 1 for simple move, 2 for swap move
	OnClose      func(reason common.EnqueueResult) // will get called when proposal is closed
}

func NewProposal(ctx context.Context, solverType string, gain Gain, basedOn SnapshotId) *Proposal {
	return &Proposal{
		ProposalId:   data.ProposalId(kcommon.RandomString(ctx, 8)),
		SolverType:   solverType,
		StartTimeMs:  kcommon.GetWallTimeMs(),
		ProposalSize: 1,
	}
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

type Move interface {
	GetSignature() string
	Apply(snapshot *Snapshot)
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

func (move *SimpleMove) Apply(snapshot *Snapshot) {
	snapshot.Unassign(move.Src, move.Replica.ShardId, move.Replica.ReplicaIdx, move.SrcAssignmentId)
	snapshot.Assign(move.Replica.ShardId, move.Replica.ReplicaIdx, move.DestAssignmentId, move.Dst)
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

func (move *AssignMove) Apply(snapshot *Snapshot) {
	snapshot.Assign(move.Replica.ShardId, move.Replica.ReplicaIdx, move.AssignmentId, move.Worker)
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

func (move *UnassignMove) Apply(snapshot *Snapshot) {
	snapshot.Unassign(move.Worker, move.Replica.ShardId, move.Replica.ReplicaIdx, move.AssignmentId)
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
