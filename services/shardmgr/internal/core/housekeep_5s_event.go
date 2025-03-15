package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
)

// Housekeep1sEvent implements krunloop.IEvent[*ServiceState] interface
type Housekeep5sEvent struct {
}

func (te *Housekeep5sEvent) GetName() string {
	return "TimerEvent"
}

func (te *Housekeep5sEvent) Process(ctx context.Context, ss *ServiceState) {
	// ss.checkWorkerTombStone(ctx)
	kcommon.ScheduleRun(1000, func() {
		ss.PostEvent(NewHousekeep5sEvent())
	})
}

func NewHousekeep5sEvent() *Housekeep5sEvent {
	return &Housekeep5sEvent{}
}
