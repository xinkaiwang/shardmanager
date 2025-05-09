package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kmetrics"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
)

var (
	acceptSoftGainMetrics = kmetrics.CreateKmetric(context.Background(), "accept_soft_gain", "soft gain of accepted proposals", []string{"solver"})
	acceptHardGainMetrics = kmetrics.CreateKmetric(context.Background(), "accept_hard_gain", "hard gain of accepted proposals", []string{"solver"})
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
	// var totalImpact costfunc.Gain
	now := kcommon.GetWallTimeMs()
	for {
		if ss.ProposalQueue.IsEmpty() {
			break
		}
		proposal := ss.ProposalQueue.Pop()
		// check 1: is proposal.BasedOn is up to date?
		if proposal.BasedOn != ss.GetSnapshotFutureForClone().SnapshotId {
			// re-evaluate the proposal gain (based on the new snapshot)
			ke := kcommon.TryCatchRun(ctx, func() {
				currentCost := ss.GetSnapshotFutureForClone().GetCost()
				newCost := ss.GetSnapshotFutureForClone().Clone().ApplyMove(proposal.Move, costfunc.AM_Strict).GetCost()
				gain := currentCost.Substract(newCost)
				proposal.Gain = gain
				proposal.BasedOn = ss.GetSnapshotFutureForClone().SnapshotId
				// add this proposal back to the queue again
				ss.ProposalQueue.Push(proposal)
			})
			if ke != nil {
				proposal.Dropped(ctx, common.ER_Conflict)
			}
			continue
		}
		// check 2: is this proposal still valid?

		// check 3: is this proposal's gain high enough (hard score)?
		gain := proposal.GetEfficiency()
		if gain.HardScore > 0 {
			ss.DoAcceptProposal(ctx, proposal)
			accpeted = append(accpeted, proposal)
			// totalImpact = totalImpact.Add(proposal.Gain)
			continue
		}
		// check 4: is this proposal's gain high enough (soft score)?
		threshold := ss.DynamicThreshold.GetCurrentThreshold(now)
		if gain.SoftScore > threshold {
			ss.DoAcceptProposal(ctx, proposal)
			accpeted = append(accpeted, proposal)
			// totalImpact = totalImpact.Add(proposal.Gain)
			continue
		} else {
			// top proposal is not good enough, so we need to wait for a while
			break
		}
	}
	// ss.AcceptedCount += len(accpeted)
	if len(accpeted) > 0 {
		future := ss.GetSnapshotFutureForAny()
		klogging.Info(ctx).With("accepted", len(accpeted)).With("future", future.SnapshotId).With("cost", future.GetCost().String()).Log("AcceptEvent", "broadcastSnapshot")
		ss.boardcastSnapshotBatchManager.TryScheduleInternal(ctx, "acceptEvent")
		// ss.broadcastSnapshot(ctx, "acceptCount="+strconv.Itoa(len(accpeted)))
	}
}

func (ss *ServiceState) DoAcceptProposal(ctx context.Context, proposal *costfunc.Proposal) {
	now := kcommon.GetWallTimeMs()
	threshold := ss.DynamicThreshold.GetCurrentThreshold(now)
	klogging.Info(ctx).With("proposalId", proposal.ProposalId).With("solverType", proposal.SolverType).With("gain", proposal.Gain).With("signature", proposal.Move.GetSignature()).With("currentThreadshold", threshold).With("base", proposal.BasedOn).Log("AcceptEvent", "接受提案")
	ss.AcceptedCount++
	acceptSoftGainMetrics.GetTimeSequence(ctx, proposal.SolverType).Add(int64(proposal.Gain.SoftScore))
	acceptHardGainMetrics.GetTimeSequence(ctx, proposal.SolverType).Add(int64(proposal.Gain.HardScore))

	moveState := NewMoveStateFromProposal(ss, proposal)
	minion := NewActionMinion(ctx, ss, moveState)
	ss.storeProvider.StoreMoveState(proposal.ProposalId, moveState.ToMoveStateJson("accepted"))
	ss.DynamicThreshold.UpdateThreshold(now, proposal.ProposalSize)
	ss.AllMoves[proposal.ProposalId] = minion

	// apply this move to future snapshot
	ke := kcommon.TryCatchRun(ctx, func() {
		newFuture := ss.GetSnapshotFutureForClone().Clone()
		newFuture.ApplyMove(proposal.Move, costfunc.AM_Strict)
		newFuture.Freeze()
		ss.SetSnapshotFuture(ctx, newFuture, "DoAcceptProposal")
	})
	if ke != nil {
		// this should not happen, but just in case
		klogging.Fatal(ctx).WithError(ke).Log("AcceptEvent", "error in applying move to future snapshot")
	}
}
