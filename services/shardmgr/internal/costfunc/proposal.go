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
	Gain        Gain
	BasedOn     SnapshotId
	StartTimeMs int64 // epoch time in ms

	// one of them is not nil
	SimpleMove  *SimpleMove
	SwapMove    *SwapMove
	ReplaceMove *ReplaceMove
}

func NewProposal(ctx context.Context) *Proposal {
	return &Proposal{
		ProposalId:  data.ProposalId(kcommon.RandomString(ctx, 8)),
		StartTimeMs: kcommon.GetWallTimeMs(),
	}
}

func (proposal *Proposal) GetSignature() string {
	if proposal.SimpleMove != nil {
		return proposal.SimpleMove.GetSignature()
	} else if proposal.SwapMove != nil {
		return proposal.SwapMove.GetSignature()
	} else if proposal.ReplaceMove != nil {
		return proposal.ReplaceMove.GetSignature()
	} else {
		return ""
	}
}

type SimpleMove struct {
	Replica data.ReplicaFullId
	Src     data.WorkerFullId
	Dst     data.WorkerFullId
}

func (move *SimpleMove) GetSignature() string {
	return move.Replica.String() + "/" + move.Src.String() + "/" + move.Dst.String()
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
	klogging.Debug(ctx).With("proposalId", prop.ProposalId).With("signature", prop.GetSignature()).Log("ProposalClosed", "dropped")
}
