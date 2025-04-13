package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
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
	var accpeted []*costfunc.Proposal
	var totalImpact costfunc.Gain
	now := kcommon.GetWallTimeMs()
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
		gain := proposal.GetEfficiency()
		if gain.HardScore > 0 {
			ss.DoAcceptProposal(ctx, proposal)
			accpeted = append(accpeted, proposal)
			totalImpact = totalImpact.Add(proposal.Gain)
			ss.DynamicThreshold.UpdateThreshold(now, proposal.ProposalSize)
			continue
		}
		// check 3: is this proposal's gain high enough (soft score)?
		threshold := ss.DynamicThreshold.GetCurrentThreshold(now)
		if gain.SoftScore > threshold {
			ss.DoAcceptProposal(ctx, proposal)
			accpeted = append(accpeted, proposal)
			totalImpact = totalImpact.Add(proposal.Gain)
			ss.DynamicThreshold.UpdateThreshold(now, proposal.ProposalSize)
			continue
		} else {
			// top proposal is not good enough, so we need to wait for a while
			break
		}
	}
	ss.AcceptedCount += len(accpeted)
}

func (ss *ServiceState) DoAcceptProposal(ctx context.Context, proposal *costfunc.Proposal) {
	threshold := ss.DynamicThreshold.GetCurrentThreshold(kcommon.GetWallTimeMs())
	klogging.Info(ctx).With("proposalId", proposal.ProposalId).With("solverType", proposal.SolverType).With("gain", proposal.Gain).With("currentThreadshold", threshold).Log("AcceptEvent", "接受提案")
	moveState := NewMoveStateFromProposal(ss, proposal)
	minion := NewActionMinion(ss, moveState)
	ss.storeProvider.StoreMoveState(proposal.ProposalId, moveState.ToMoveStateJson("accepted"))
	ss.AllMoves[proposal.ProposalId] = minion
	go minion.Run(ctx, ss)
}
