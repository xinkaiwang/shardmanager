package core

import (
	"context"
	"sync"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
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

func (bm *BatchManager) TrySchedule(ctx context.Context) {
	var needSchedule bool
	bm.mutex.Lock()
	if !bm.isInFlight {
		bm.isInFlight = true
		needSchedule = true
	}
	bm.mutex.Unlock()
	scheduled := "scheduled"
	if !needSchedule {
		scheduled = "merged"
	}
	klogging.Info(ctx).With("name", bm.name).With("result", scheduled).Log("BatchManager", "in-bound")
	if !needSchedule {
		return
	}
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

	klogging.Info(ctx).With("name", bpe.parent.name).Log("BatchManager", "out-bound")
	bpe.parent.fn(ctx, ss)
}
