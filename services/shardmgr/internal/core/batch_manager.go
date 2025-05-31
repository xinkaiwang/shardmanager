package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/libs/xklib/krunloop"
)

type BatchManager struct {
	parent     krunloop.EventPoster[*ServiceState]
	maxDelayMs int
	name       string
	isInFlight bool // true means the event is already being scheduled
	reasons    []string
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

func (bm *BatchManager) TrySchedule(ctx context.Context, reasons ...string) {
	krunloop.VisitResource(bm.parent, func(ss *ServiceState) {
		bm.TryScheduleInternal(ctx, reasons...)
	})
}

// TryScheduleInternal must be called from inside runloop
func (bm *BatchManager) TryScheduleInternal(ctx context.Context, reasons ...string) {
	var needSchedule bool
	if !bm.isInFlight {
		bm.isInFlight = true
		needSchedule = true
	}
	bm.reasons = append(bm.reasons, reasons...)
	scheduled := "scheduled"
	if !needSchedule {
		scheduled = "merged"
	}
	klogging.Info(ctx).With("name", bm.name).With("result", scheduled).With("reason", reasons).Log("BatchManager", "in-bound")
	if !needSchedule {
		return
	}
	kcommon.ScheduleRun(bm.maxDelayMs, func() {
		bm.parent.PostEvent(NewBatchProcessEvent(ctx, bm))
	})
}

// BatchProcessEvent implements krunloop.IEvent interface
type BatchProcessEvent struct {
	ctx    context.Context
	parent *BatchManager
}

func NewBatchProcessEvent(ctx context.Context, parent *BatchManager) *BatchProcessEvent {
	return &BatchProcessEvent{
		ctx:    ctx,
		parent: parent,
	}
}

func (bpe *BatchProcessEvent) GetName() string {
	return bpe.parent.name
}

func (bpe *BatchProcessEvent) Process(_ context.Context, ss *ServiceState) {
	var reasons []string
	bpe.parent.isInFlight = false
	reasons = bpe.parent.reasons
	bpe.parent.reasons = nil

	klogging.Info(bpe.ctx).With("name", bpe.parent.name).With("reason", reasons).Log("BatchManager", "out-bound")
	bpe.parent.fn(bpe.ctx, ss)
}
