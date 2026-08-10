package krunloop

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// traceCaptureEvent 在 Process 时捕获 ctx 里的 SpanContext
type traceCaptureEvent struct {
	wg  *sync.WaitGroup
	out *trace.SpanContext // 写入捕获结果
}

func (e *traceCaptureEvent) GetCreateTimeMs() int64 { return 0 }
func (e *traceCaptureEvent) GetName() string        { return "TraceCapture" }
func (e *traceCaptureEvent) Process(ctx context.Context, _ *TestRunLoopResource) {
	*e.out = trace.SpanFromContext(ctx).SpanContext()
	e.wg.Done()
}

// KLOG-011：每个 runloop 事件必须在自己的 root span 里执行——
// Process 的 ctx 携带有效 span，且不同事件的 trace_id 互不相同（各自独立成 trace）。
func TestRunLoop_EachEventGetsOwnRootTrace(t *testing.T) {
	tp := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	otel.SetTracerProvider(tp)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	ctx := context.Background()
	rl := NewRunLoop(ctx, &TestRunLoopResource{}, "test-loop")
	go rl.Run(ctx)
	defer rl.StopAndWaitForExit()

	var sc1, sc2 trace.SpanContext
	var wg sync.WaitGroup
	wg.Add(2)
	rl.PostEvent(&traceCaptureEvent{wg: &wg, out: &sc1})
	rl.PostEvent(&traceCaptureEvent{wg: &wg, out: &sc2})

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("events not processed in time")
	}

	if !sc1.IsValid() || !sc2.IsValid() {
		t.Fatalf("event Process ctx must carry a valid span: sc1=%+v sc2=%+v", sc1, sc2)
	}
	if sc1.TraceID() == sc2.TraceID() {
		t.Errorf("each event must be its own root trace, both got trace_id=%s", sc1.TraceID())
	}
}
