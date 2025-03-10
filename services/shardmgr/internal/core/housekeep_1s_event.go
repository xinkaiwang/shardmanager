package core

import (
	"context"
)

// Housekeep1sEvent implements krunloop.IEvent[*ServiceState] interface
type Housekeep1sEvent struct {
}

func (te *Housekeep1sEvent) GetName() string {
	return "TimerEvent"
}

func (te *Housekeep1sEvent) Process(ctx context.Context, ss *ServiceState) {
	ss.checkWorkerForTimeout(ctx)
}

func (ss *ServiceState) checkWorkerForTimeout(ctx context.Context) {
	for workerFullId, worker := range ss.AllWorkers {
		needsDelete := worker.checkWorkerForTimeout(ctx, ss)
		if needsDelete {
			delete(ss.AllWorkers, workerFullId)
		}
	}
}
