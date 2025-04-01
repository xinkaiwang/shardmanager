package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
)

// AcceptEvent implements krunloop.IEvent[*ServiceState] interface
type AcceptEvent struct {
}

func NewAcceptEvent() *AcceptEvent {
	return &AcceptEvent{}
}

func (te *AcceptEvent) GetName() string {
	return "AcceptEvent"
}

func (te *AcceptEvent) Process(ctx context.Context, ss *ServiceState) {
	ss.TryAccept(ctx)
	kcommon.ScheduleRun(500, func() {
		ss.PostEvent(NewAcceptEvent())
	})
}

func (ss *ServiceState) TryAccept(ctx context.Context) {
	for {
		if ss.ProposalQueue.IsEmpty() {
			break
		}
		proposal := ss.ProposalQueue.Pop()
		// check 1: is proposal.BasedOn is up to date?
		if proposal.BasedOn != ss.SnapshotFuture.SnapshotId {
			// re-evaluate the proposal gain (based on the new snapshot)
			ke := kcommon.TryCatchRun(ctx, func() {
				currentCost := ss.SnapshotFuture.GetCost()
				newCost := ss.SnapshotFuture.Clone().ApplyMove(proposal.Move).GetCost()
				gain := currentCost.Substract(newCost)
				proposal.Gain = gain
				proposal.BasedOn = ss.SnapshotFuture.SnapshotId
				// add this proposal back to the queue again
				ss.ProposalQueue.Push(proposal)
			})
			if ke != nil {
				proposal.Dropped(ctx, common.ER_Conflict)
			}
			continue
		}
		// check 2: is this proposal's gain high enough (hard score)?
		if proposal.Gain.HardScore > 0 {
			ss.DoAcceptProposal(ctx, proposal)
			continue
		}
		// check 3: is this proposal's gain high enough (soft score)?
		// threshold :=
	}
}

func (ss *ServiceState) DoAcceptProposal(ctx context.Context, proposal *costfunc.Proposal) {
	// TODO
	ss.AcceptedCount++
}
