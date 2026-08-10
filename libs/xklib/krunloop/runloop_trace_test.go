package krunloop

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
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

// KLOG-011 两级身份之二：事件处理期间的日志自动带 runloop=<name>（ambient attr）
func TestRunLoop_LogsCarryRunloopName(t *testing.T) {
	var buf bytes.Buffer
	h := klogging.NewHandler(&klogging.HandlerOptions{Level: slog.LevelInfo, Format: "json", Output: &buf})
	old := slog.Default()
	slog.SetDefault(slog.New(h))
	defer slog.SetDefault(old)

	ctx := context.Background()
	rl := NewRunLoop(ctx, &TestRunLoopResource{}, "core-loop")
	go rl.Run(ctx)
	defer rl.StopAndWaitForExit()

	var wg sync.WaitGroup
	wg.Add(1)
	rl.PostEvent(&logEmittingEvent{wg: &wg})

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("event not processed in time")
	}

	var m map[string]interface{}
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &m); err != nil {
		t.Fatalf("bad json: %v: %s", err, buf.String())
	}
	if m["runloop"] != "core-loop" {
		t.Errorf("event log must carry runloop name, got: %v", m)
	}
}

type logEmittingEvent struct{ wg *sync.WaitGroup }

func (e *logEmittingEvent) GetCreateTimeMs() int64 { return 0 }
func (e *logEmittingEvent) GetName() string        { return "LogEmitting" }
func (e *logEmittingEvent) Process(ctx context.Context, _ *TestRunLoopResource) {
	slog.InfoContext(ctx, "inside event", slog.String("event", "InsideEvent"))
	e.wg.Done()
}
