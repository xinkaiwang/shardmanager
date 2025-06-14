package krunloop

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kmetrics"
)

var (
	RunLoopElapsedMsMetric   = kmetrics.CreateKmetric(context.Background(), "runloop_elapsed_ms", "desc", []string{"name", "event"})
	RunLoopQueueTimeMsMetric = kmetrics.CreateKmetric(context.Background(), "runloop_queue_time_ms", "desc", []string{"name", "event"})
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
	GetCreateTimeMs() int64 // for metrics/debugging purposes, returns the time when the event was enqueue/created
}

type EventPoster[T CriticalResource] interface {
	PostEvent(event IEvent[T])
}

// RunLoop: implements EventPoster interface
// RunLoop is a generic event processing loop for any resource type
type RunLoop[T CriticalResource] struct {
	name             string // name of this runloop: for logging/metrics purposes only
	resource         T
	queue            *UnboundedQueue[T]
	currentEventName atomic.Value // 使用原子操作保护事件名
	sampler          *RunloopSampler
	mu               sync.Mutex // 保护 ctx 和 cancel
	ctx              context.Context
	cancel           context.CancelFunc
	epochId          int64 // 事件循环的时间戳

	stop    chan struct{} // 用于停止 RunLoop
	stopped chan struct{}
}

// NewRunLoop creates a new RunLoop for the given resource.
// name is used for logging/metrics purposes only
func NewRunLoop[T CriticalResource](ctx context.Context, resource T, name string) *RunLoop[T] {
	rl := &RunLoop[T]{
		name:     name,
		resource: resource,
		queue:    NewUnboundedQueue[T](ctx),
		epochId:  0,
		stop:     make(chan struct{}), // 初始化 stop 通道
		stopped:  make(chan struct{}),
	}
	rl.sampler = NewRunloopSampler(ctx, func() string {
		val := rl.currentEventName.Load()
		if val == nil {
			return ""
		}
		return val.(string)
	}, name)
	return rl
}

// PostEvent: Enqueue an event to the run loop. This call never blocks.
func (rl *RunLoop[T]) PostEvent(event IEvent[T]) {
	rl.queue.Enqueue(event)
}

func (rl *RunLoop[T]) GetNextEpochId() string {
	// 使用原子操作获取 epochId
	id := atomic.AddInt64(&rl.epochId, 1)
	return fmt.Sprintf("%d", id)
}

func (rl *RunLoop[T]) GetQueueLength() int {
	// 获取队列长度
	return int(rl.queue.GetSize())
}
func (rl *RunLoop[T]) GetName() string {
	return rl.name
}
func (rl *RunLoop[T]) Run(ctx context.Context) {
	// 使用互斥锁保护 ctx 和 cancel 的设置
	rl.mu.Lock()
	rl.ctx, rl.cancel = context.WithCancel(ctx)
	rl.mu.Unlock()

	defer func() {
		// 通知 RunLoop 已退出
		close(rl.stopped)
	}()

	stop := false
	for !stop {
		select {
		case <-rl.ctx.Done():
			klogging.Info(ctx).Log("RunLoopCtxCanceled", "run loop stopped")
			stop = true
			continue
		case event, ok := <-rl.queue.GetOutputChan():
			if !ok {
				klogging.Info(ctx).Log("EventQueueClosed", "event queue closed")
				stop = true
				continue
			}
			// Handle event
			start := kcommon.GetMonoTimeMs()
			eveName := event.GetName()
			if eveName == "" {
				eveName = "unknown"
			}
			waitTimeMs := kcommon.GetWallTimeMs() - event.GetCreateTimeMs()
			RunLoopQueueTimeMsMetric.GetTimeSequence(ctx, rl.name, eveName).Add(waitTimeMs)
			// 使用原子操作存储当前事件名
			rl.currentEventName.Store(eveName)
			ctx2 := klogging.EmbedTraceId(ctx, "rl_"+rl.GetNextEpochId())
			event.Process(ctx2, rl.resource)
			rl.currentEventName.Store("")
			elapsedMs := kcommon.GetMonoTimeMs() - start
			RunLoopElapsedMsMetric.GetTimeSequence(ctx, rl.name, eveName).Add(elapsedMs)
		case <-rl.stop:
			stop = true
		}
	}
}

func (rl *RunLoop[T]) StopAndWaitForExit() {
	// 使用互斥锁保护对 cancel 的访问
	rl.mu.Lock()
	cancel := rl.cancel
	rl.cancel = nil
	rl.mu.Unlock()

	// 如果 cancel 为 nil，则 runloop 尚未启动，无需等待
	if cancel == nil {
		return
	}

	rl.queue.StopAndWaitForExit()
	// 取消 context
	cancel()

	// 设置短超时，避免无限等待
	select {
	case <-rl.stopped:
		// 正常退出
	case <-time.After(1000 * time.Millisecond): // 增加超时时间，确保有足够时间退出
		// 超时，可能 Run 方法尚未完全启动或已异常退出
		klogging.Warning(context.Background()).Log("RunLoopStopTimeout", "RunLoop.StopAndWaitForExit 超时")
	}
}

func (rl *RunLoop[T]) InitTimeSeries(ctx context.Context, names ...string) {
	// 初始化时间序列, 提供0值
	rl.sampler.InitTimeSeries(ctx, names...)
	// 初始化时间序列, 提供0值
	for _, name := range names {
		RunLoopElapsedMsMetric.GetTimeSequence(ctx, rl.name, name).Touch()
	}
}

// ResourceVisitorEvent implements IEvent[T] interface
type ResourceVisitorEvent[T CriticalResource] struct {
	createTimeMs int64 // time when the event was created
	callback     func(res T)
}

func NewServiceStateVisitorEvent[T CriticalResource](callback func(res T)) *ResourceVisitorEvent[T] {
	return &ResourceVisitorEvent[T]{
		createTimeMs: kcommon.GetWallTimeMs(),
		callback:     callback,
	}
}
func (e *ResourceVisitorEvent[T]) GetCreateTimeMs() int64 {
	return e.createTimeMs
}
func (e *ResourceVisitorEvent[T]) GetName() string {
	return "ResourceVisitor"
}
func (e *ResourceVisitorEvent[T]) Process(ctx context.Context, resource T) {
	e.callback(resource)
}
func VisitResource[T CriticalResource](poster EventPoster[T], callback func(res T)) {
	eve := NewServiceStateVisitorEvent(func(res T) {
		callback(res)
	})
	poster.PostEvent(eve)
}
func VisitResourceAndWait[T CriticalResource](poster EventPoster[T], callback func(res T)) {
	ch := make(chan struct{})
	eve := NewServiceStateVisitorEvent(func(res T) {
		callback(res)
		close(ch)
	})
	poster.PostEvent(eve)
	<-ch
}
