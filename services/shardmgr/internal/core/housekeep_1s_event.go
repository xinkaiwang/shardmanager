package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
)

// Housekeep1sEvent implements krunloop.IEvent[*ServiceState] interface
type Housekeep1sEvent struct {
}

func (te *Housekeep1sEvent) GetName() string {
	return "TimerEvent"
}

func (te *Housekeep1sEvent) Process(ctx context.Context, ss *ServiceState) {
	ke := kcommon.TryCatchRun(ctx, func() {
		ss.checkWorkerForTimeout(ctx)
	})
	if ke != nil {
		klogging.Error(ctx).With("error", ke).Log("Housekeep1sEvent", "checkWorkerForTimeout failed")
	}
	kcommon.ScheduleRun(1000, func() {
		ss.PostEvent(NewHousekeep1sEvent())
	})
}

func NewHousekeep1sEvent() *Housekeep1sEvent {
	return &Housekeep1sEvent{}
}

func (ss *ServiceState) checkWorkerForTimeout(ctx context.Context) {
	var passiveMoves []PassiveMove
	// check whether workers needs hats
	for workerFullId, worker := range ss.AllWorkers {
		dirty := worker.checkWorkerForHat(ctx)
		if dirty {
			move := NewPassiveMoveWorkerGotHat(workerFullId)
			passiveMoves = append(passiveMoves, move)
		}
		needsDelete := worker.checkWorkerOnTimeout(ctx, ss)
		if needsDelete {
			delete(ss.AllWorkers, workerFullId)
		}
	}
	if len(passiveMoves) > 0 {
		// post passive moves
		for _, move := range passiveMoves {
			ss.ModifySnapshot(ctx, move.Apply, move.Signature())
		}
	}
}
