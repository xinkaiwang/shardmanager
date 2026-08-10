package krunloop

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kmetrics"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

var (
	// in/out 三角（XS-007）：enqueue_ct = in，elapsed_ms 的 count = out，
	// 队列积压深度 = runloop_enqueue_ct_count − runloop_elapsed_ms_count（PromQL 可算）。
	RunLoopElapsedMsMetric   = kmetrics.CreateKmetric(context.Background(), "runloop_elapsed_ms", "per-event processing time (count = events processed, sum = ms)", []string{"name", "event"})
	RunLoopQueueTimeMsMetric = kmetrics.CreateKmetric(context.Background(), "runloop_queue_time_ms", "per-event wait in queue before processing (count = events dequeued, sum = ms)", []string{"name", "event"})
	RunLoopEnqueueMetric     = kmetrics.CreateKmetric(context.Background(), "runloop_enqueue_ct", "events enqueued (queue in-side; backlog = enqueue_ct_count - elapsed_ms_count)", []string{"name", "event"}).CountOnly()
	// drop 边（XS-007 三角补全）：停机后到达的事件被响亮丢弃计数
	RunLoopEnqueueDroppedMetric = kmetrics.CreateKmetric(context.Background(), "runloop_enqueue_dropped_ct", "events dropped because they arrived after queue stop (shutdown stragglers)", []string{"event"}).CountOnly()

	// 包级缓存，避免每事件过 TracerProvider.Tracer() 的 mutex+map（KLOG-005b ③-5）。
	// otel.Tracer 返回的是动态代理：每次 Start 时解析当前全局 provider，无初始化顺序问题。
	runloopTracer = otel.Tracer("xklib/krunloop")
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
	RunLoopEnqueueMetric.GetTimeSequence(context.Background(), rl.name, event.GetName()).Add(1)
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
	// KLOG-011 两级身份之二：runloop 身份走 ambient attr（人类可读、跨重启稳定），
	// "哪次 run" 走每事件 trace_id。此后本循环内所有日志自动带 runloop=<name>。
	ctx = klogging.CtxWithAttrs(ctx, slog.String("runloop", rl.name))

	// 使用互斥锁保护 ctx 和 cancel 的设置
	rl.mu.Lock()
	rl.ctx, rl.cancel = context.WithCancel(ctx)
	rl.mu.Unlock()

	defer func() {
		// XS-002 权威收尾：Run 退出 ⇒ queue 必须停止（closed 置位，Enqueue 转为
		// 响亮拒绝）。此前这依赖"调用方给 NewRunLoop 和 Run 传同一个 ctx"的
		// 隐式约定——约定破裂时 queue 存活、事件静默入 buffer 无人消费。
		rl.queue.Stop()
		// 通知 RunLoop 已退出
		close(rl.stopped)
	}()

	stop := false
	for !stop {
		select {
		case <-rl.ctx.Done():
			slog.InfoContext(ctx, "run loop stopped",
				slog.String("event", "RunLoopCtxCanceled"))
			stop = true
			continue
		case event, ok := <-rl.queue.GetOutputChan():
			if !ok {
				slog.InfoContext(ctx, "event queue closed",
					slog.String("event", "EventQueueClosed"))
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
			// KLOG-011: runloop 是 daemon，每个事件在自己的 root span 里执行
			// （WithNewRoot 显式声明：不继承 Run 的 ctx 里可能存在的任何 span）。
			// 事件处理期间的所有日志由此获得同一 trace_id；采样按事件独立决策。
			// 前提：main 已调用 klogging.InitDefaultTracerProvider，否则退化为 noop。
			ctx2, span := runloopTracer.Start(ctx, eveName, trace.WithNewRoot())
			event.Process(ctx2, rl.resource)
			span.End()
			rl.currentEventName.Store("")
			elapsedMs := kcommon.GetMonoTimeMs() - start
			RunLoopElapsedMsMetric.GetTimeSequence(ctx, rl.name, eveName).Add(elapsedMs)
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
	// XS-005: 停止 sampler 的自我重调度链（否则每个 RunLoop 泄漏一条永久 50Hz 定时器链）
	rl.sampler.Stop()

	// XS-006: 等到 Run 真正退出为止，不设放弃超时——旧的"等 1s 然后放弃返回"
	// 会让调用方在事件仍在处理时开始拆资源（use-after-teardown）。改为无限等待 +
	// 周期性进度 Warn（带当前事件名，operator 可判断是哪个事件卡住）。
	// 1s 只是日志节奏，不承重。
	for {
		select {
		case <-rl.stopped:
			return // 正常退出
		case <-time.After(1000 * time.Millisecond):
			eveName, _ := rl.currentEventName.Load().(string)
			slog.WarnContext(context.Background(), "RunLoop.StopAndWaitForExit still waiting for in-flight event",
				slog.String("event", "RunLoopStopWaiting"),
				slog.String("runloop", rl.name),
				slog.String("currentEvent", eveName))
		}
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
