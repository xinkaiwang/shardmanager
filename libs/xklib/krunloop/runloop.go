package krunloop

import (
	"context"
	"time"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kmetrics"
)

var (
	RunLoopElapsedMsMetric = kmetrics.CreateKmetric(context.Background(), "runloop_elapsed_ms", "desc", []string{"name", "event"})
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
	name             string // name of this runloop: for logging/metrics purposes only
	resource         T
	queue            *UnboundedQueue[T]
	currentEventName string
	sampler          *RunloopSampler
	ctx              context.Context
	cancel           context.CancelFunc
	exited           chan struct{}
}

// NewRunLoop creates a new RunLoop for the given resource.
// name is used for logging/metrics purposes only
func NewRunLoop[T CriticalResource](ctx context.Context, resource T, name string) *RunLoop[T] {
	rl := &RunLoop[T]{
		name:     name,
		resource: resource,
		queue:    NewUnboundedQueue[T](ctx),
		exited:   make(chan struct{}),
	}
	rl.sampler = NewRunloopSampler(ctx, func() string { return rl.currentEventName }, name)
	return rl
}

// EnqueueEvent: Enqueue an event to the run loop. This call never blocks.
func (rl *RunLoop[T]) EnqueueEvent(event IEvent[T]) {
	rl.queue.Enqueue(event)
}

func (rl *RunLoop[T]) Run(ctx context.Context) {
	rl.ctx, rl.cancel = context.WithCancel(ctx)
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
				RunLoopElapsedMsMetric.GetTimeSequence(ctx, rl.name, eveName).Add(elapsedMs)
			}()
			event.Process(ctx, rl.resource)
		}
	}
}

func (rl *RunLoop[T]) StopAndWaitForExit() {
	// 如果 cancel 为 nil，则 runloop 尚未启动，无需等待
	if rl.cancel == nil {
		return
	}

	// 取消 context
	rl.cancel()

	// 设置短超时，避免无限等待
	select {
	case <-rl.exited:
		// 正常退出
	case <-time.After(100 * time.Millisecond):
		// 超时，可能 Run 方法尚未完全启动或已异常退出
	}
}
