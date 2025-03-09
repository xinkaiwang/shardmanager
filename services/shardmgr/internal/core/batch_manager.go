package core

import (
	"context"
	"sync"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/krunloop"
)

type BatchManager struct {
	mutex      sync.Mutex
	parent     krunloop.EventPoster[*ServiceState]
	maxDelayMs int
	name       string
	isInFlight bool // true means the event is already being scheduled
	fn         func(context.Context, *ServiceState)
}

func NewBatchManager(parent krunloop.EventPoster[*ServiceState], maxDelayMs int, name string, fn func(context.Context, *ServiceState)) *BatchManager {
	return &BatchManager{
		parent:     parent,
		maxDelayMs: maxDelayMs,
		name:       name,
		fn:         fn,
	}
}

func (bm *BatchManager) TrySchedule() {
	bm.mutex.Lock()
	defer bm.mutex.Unlock()
	if bm.isInFlight {
		return
	}
	bm.isInFlight = true
	kcommon.ScheduleRun(bm.maxDelayMs, func() {
		bm.parent.PostEvent(NewBatchProcessEvent(bm))
	})
}

// BatchProcessEvent implements krunloop.IEvent interface
type BatchProcessEvent struct {
	parent *BatchManager
}

func NewBatchProcessEvent(parent *BatchManager) *BatchProcessEvent {
	return &BatchProcessEvent{
		parent: parent,
	}
}

func (bpe *BatchProcessEvent) GetName() string {
	return bpe.parent.name
}

func (bpe *BatchProcessEvent) Process(ctx context.Context, ss *ServiceState) {
	bpe.parent.mutex.Lock()
	bpe.parent.isInFlight = false
	bpe.parent.mutex.Unlock()

	bpe.parent.fn(ctx, ss)
}
