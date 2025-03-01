package krunloop

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kmetrics"
)

var (
	RunLoopElapsedMsMetric = kmetrics.CreateKmetric(context.Background(), "runloop_elapsed_ms", "desc", []string{"event"})
)

// CriticalResource is an interface that represents resources that can be processed by events
// in a RunLoop. This provides better type safety than using 'any'.
type CriticalResource interface {
	// IsResource is a marker method to identify types that can be used as critical resources
	IsResource()
}

// IEvent is a generic interface for events that can be processed by a RunLoop
type IEvent[T CriticalResource] interface {
	GetName() string
	Process(ctx context.Context, resource T)
}

// RunLoop is a generic event processing loop for any resource type
type RunLoop[T CriticalResource] struct {
	resource         T
	queue            *UnboundedQueue[T]
	currentEventName string
	sampler          *RunloopSampler
}

// NewRunLoop creates a new RunLoop for the given resource
func NewRunLoop[T CriticalResource](ctx context.Context, resource T) *RunLoop[T] {
	rl := &RunLoop[T]{
		resource: resource,
		queue:    NewUnboundedQueue[T](ctx),
	}
	rl.sampler = NewRunloopSampler(ctx, func() string { return rl.currentEventName })
	return rl
}

// EnqueueEvent: Enqueue an event to the run loop. This call never blocks.
func (rl *RunLoop[T]) EnqueueEvent(event IEvent[T]) {
	rl.queue.Enqueue(event)
}

func (rl *RunLoop[T]) Run(ctx context.Context) {
	defer rl.queue.Close()

	for {
		select {
		case <-ctx.Done():
			klogging.Info(ctx).Log("RunLoopCtxCanceled", "run loop stopped")
			return
		case event, ok := <-rl.queue.GetOutputChan():
			if !ok {
				klogging.Info(ctx).Log("EventQueueClosed", "event queue closed")
				return
			}
			// Handle event
			start := kcommon.GetMonoTimeMs()
			eveName := event.GetName()
			rl.currentEventName = eveName
			defer func() {
				rl.currentEventName = ""
				elapsedMs := kcommon.GetMonoTimeMs() - start
				RunLoopElapsedMsMetric.GetTimeSequence(ctx, eveName).Add(elapsedMs)
			}()
			event.Process(ctx, rl.resource)
		}
	}
}
