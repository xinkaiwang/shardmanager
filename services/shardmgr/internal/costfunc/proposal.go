package costfunc

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

// Proposal implements OrderedListItem
type Proposal struct {
	ProposalId  data.ProposalId
	SolverType  string
	Gain        Gain
	BasedOn     SnapshotId
	StartTimeMs int64 // epoch time in ms

	// one of them is not nil
	SimpleMove  *SimpleMove
	AssignMove  *AssignMove
	SwapMove    *SwapMove
	ReplaceMove *ReplaceMove
	OnClose     func(reason common.EnqueueResult) // will get called when proposal is closed
}

func NewProposal(ctx context.Context, solverType string, gain Gain, basedOn SnapshotId) *Proposal {
	return &Proposal{
		ProposalId:  data.ProposalId(kcommon.RandomString(ctx, 8)),
		SolverType:  solverType,
		StartTimeMs: kcommon.GetWallTimeMs(),
	}
}

func (proposal *Proposal) GetSignature() string {
	if proposal.SimpleMove != nil {
		return proposal.SimpleMove.GetSignature()
	} else if proposal.AssignMove != nil {
		return proposal.AssignMove.GetSignature()
	} else if proposal.SwapMove != nil {
		return proposal.SwapMove.GetSignature()
	} else if proposal.ReplaceMove != nil {
		return proposal.ReplaceMove.GetSignature()
	} else {
		return ""
	}
}

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
	srcWorker := snapshot.AllWorkers[move.Src]
	dstWorker := snapshot.AllWorkers[move.Dst]
	delete(srcWorker.Assignments, move.Replica.ShardId)
	delete(snapshot.AllAssigns, move.SrcAssignmentId)
	snapshot.AllAssigns[move.DestAssignmentId] = &AssignmentSnap{
		ShardId:      move.Replica.ShardId,
		ReplicaIdx:   move.Replica.ReplicaIdx,
		AssignmentId: move.DestAssignmentId,
	}
	dstWorker.Assignments[move.Replica.ShardId] = move.DestAssignmentId
}

type AssignMove struct {
	Replica      data.ReplicaFullId
	AssignmentId data.AssignmentId
	Worker       data.WorkerFullId
}

func (move *AssignMove) GetSignature() string {
	return move.Replica.String() + "/" + move.Worker.String()
}

func (move *AssignMove) Apply(snapshot *Snapshot) {
	snapshot.AllAssigns[move.AssignmentId] = &AssignmentSnap{
		ShardId:      move.Replica.ShardId,
		ReplicaIdx:   move.Replica.ReplicaIdx,
		AssignmentId: move.AssignmentId,
	}
	worker := snapshot.AllWorkers[move.Worker]
	worker.Assignments[move.Replica.ShardId] = move.AssignmentId
	shard := snapshot.AllShards[move.Replica.ShardId]
	shard.Replicas[move.Replica.ReplicaIdx].Assignments[move.AssignmentId] = common.Unit{}
}

type SwapMove struct {
	Replica1 data.ReplicaFullId
	Replica2 data.ReplicaFullId
	Src      data.WorkerFullId
	Dst      data.WorkerFullId
}

func (move *SwapMove) GetSignature() string {
	return move.Replica1.String() + "/" + move.Replica2.String() + "/" + move.Src.String() + "/" + move.Dst.String()
}

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
