package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
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
		if proposal.BasedOn != ss.SnapshotFuture.SnapshotId {
			// re-evaluate the proposal gain (based on the new snapshot)
			newSnapshot := ss.SnapshotFuture.Clone()
			proposal.Move.Apply(newSnapshot)
			// oldCost := ss.SnapshotFuture.GetCost()
			// TODO
		}
	}
}
