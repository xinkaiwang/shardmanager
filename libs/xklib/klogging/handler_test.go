package klogging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestHandler_Basic(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&HandlerOptions{
		Level:  slog.LevelInfo,
		Format: "json",
		Output: &buf,
	})

	logger := slog.New(handler)
	logger.Info("test message", slog.String("event", "TestEvent"), slog.String("key", "value"))

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if logEntry["event"] != "TestEvent" {
		t.Errorf("Expected event=TestEvent, got %v", logEntry["event"])
	}
	if logEntry["key"] != "value" {
		t.Errorf("Expected key=value, got %v", logEntry["key"])
	}
}

func TestHandler_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&HandlerOptions{
		Level:  slog.LevelInfo,
		Format: "json",
		Output: &buf,
	})

	logger := slog.New(handler)
	ctx := context.Background()

	// Debug should not log (below Info level)
	logger.DebugContext(ctx, "debug message")
	if buf.Len() > 0 {
		t.Error("Debug should not log when level=Info")
	}

	// Info should log
	logger.InfoContext(ctx, "info message")
	if buf.Len() == 0 {
		t.Error("Info should log when level=Info")
	}
}

func TestHandler_Sampling(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&HandlerOptions{
		Level:        slog.LevelInfo,
		SampledLevel: slog.LevelDebug,
		Format:       "json",
		Output:       &buf,
	})

	logger := slog.New(handler)
	ctx := context.Background()

	// Normal request: Debug should not log
	logger.DebugContext(ctx, "debug message")
	if buf.Len() > 0 {
		t.Error("Debug should not log without sampling")
	}

	// Sampled request: Debug should log
	ctx = trace.ContextWithSpan(ctx, &mockSampledSpan{})
	logger.DebugContext(ctx, "debug message")
	if buf.Len() == 0 {
		t.Error("Debug should log when sampled")
	}
}

func TestHandler_TraceInjection(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&HandlerOptions{
		Level:  slog.LevelInfo,
		Format: "json",
		Output: &buf,
	})

	logger := slog.New(handler)
	ctx := trace.ContextWithSpan(context.Background(), &mockSampledSpan{})

	logger.InfoContext(ctx, "test message")

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if logEntry["trace_id"] == nil {
		t.Error("Expected trace_id to be injected")
	}
	if logEntry["span_id"] == nil {
		t.Error("Expected span_id to be injected")
	}
}

func TestHandler_Metrics(t *testing.T) {
	var buf bytes.Buffer
	reporter := &mockMetricsReporter{}
	handler := NewHandler(&HandlerOptions{
		Level:   slog.LevelInfo,
		Format:  "json",
		Output:  &buf,
		Metrics: reporter,
	})

	logger := slog.New(handler)
	ctx := context.Background()

	logger.InfoContext(ctx, "test message", slog.String("event", "TestEvent"))

	// KLOG-002 支点 B：一条被输出的日志恰好上报一次，dropped=false，全字段
	if reporter.callCount() != 1 {
		t.Fatalf("emitted log must report exactly once, got %d calls", reporter.callCount())
	}
	got := reporter.last()
	if got.event != "TestEvent" {
		t.Errorf("Expected event=TestEvent, got %s", got.event)
	}
	if got.level != "INFO" {
		t.Errorf("Expected level=INFO, got %s", got.level)
	}
	if got.dropped {
		t.Error("Expected dropped=false for emitted log")
	}
	if got.size <= 0 {
		t.Errorf("Expected size>0 for emitted log, got %d", got.size)
	}
}

// KLOG-002：被级别压掉的日志必须以 dropped=true 上报（level 粒度；
// 此时拿不到 event/attrs——结构性约束，event 为空串、size 为 0）。
func TestHandler_MetricsCountsDropped(t *testing.T) {
	var buf bytes.Buffer
	reporter := &mockMetricsReporter{}
	handler := NewHandler(&HandlerOptions{
		Level:   slog.LevelInfo,
		Format:  "json",
		Output:  &buf,
		Metrics: reporter,
	})
	logger := slog.New(handler)

	logger.DebugContext(context.Background(), "suppressed", slog.String("event", "Invisible"))

	if buf.Len() != 0 {
		t.Fatalf("debug log must be suppressed at info level, got output: %s", buf.String())
	}
	if reporter.callCount() != 1 {
		t.Fatalf("dropped log must report exactly once, got %d calls", reporter.callCount())
	}
	got := reporter.last()
	if !got.dropped {
		t.Error("Expected dropped=true")
	}
	if got.level != "DEBUG" {
		t.Errorf("Expected level=DEBUG, got %s", got.level)
	}
	if got.event != "" || got.size != 0 {
		t.Errorf("dropped report cannot know event/size, got event=%q size=%d", got.event, got.size)
	}
}

// KLOG-010：运行时改级别立即生效，且对 WithAttrs 派生出的 handler 同样生效
func TestHandler_SetLevelRuntime(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&HandlerOptions{
		Level:  slog.LevelInfo,
		Format: "json",
		Output: &buf,
	})
	derived := slog.New(handler.WithAttrs([]slog.Attr{slog.String("svc", "x")}))
	base := slog.New(handler)
	ctx := context.Background()

	base.DebugContext(ctx, "before")
	derived.DebugContext(ctx, "before-derived")
	if buf.Len() != 0 {
		t.Fatalf("debug must be suppressed before SetLevel, got: %s", buf.String())
	}

	handler.SetLevel(LevelDebug)

	base.DebugContext(ctx, "after")
	if buf.Len() == 0 {
		t.Error("debug must be emitted after SetLevel(debug)")
	}
	buf.Reset()
	derived.DebugContext(ctx, "after-derived")
	if buf.Len() == 0 {
		t.Error("SetLevel must also affect handlers derived via WithAttrs")
	}
}

// Mock types

type mockSampledSpan struct {
	trace.Span
}

func (s *mockSampledSpan) SpanContext() trace.SpanContext {
	return trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanID:     trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8},
		TraceFlags: trace.FlagsSampled,
	})
}

type reportedLog struct {
	level   string
	event   string
	size    int
	dropped bool
}

type mockMetricsReporter struct {
	calls []reportedLog
}

func (r *mockMetricsReporter) ReportLog(ctx context.Context, level, event string, size int, dropped bool) {
	r.calls = append(r.calls, reportedLog{level: level, event: event, size: size, dropped: dropped})
}

func (r *mockMetricsReporter) callCount() int { return len(r.calls) }
func (r *mockMetricsReporter) last() reportedLog {
	if len(r.calls) == 0 {
		return reportedLog{}
	}
	return r.calls[len(r.calls)-1]
}

// KLOG-006: sampled 提级只能放宽不能收紧——全局级别比 sampledLevel 更低（更啰嗦）时，
// 被采样的请求必须取 min(globalLevel, sampledLevel)，不得反而打得更少。
func TestHandler_SampledNeverRaisesEffectiveLevel(t *testing.T) {
	var buf bytes.Buffer
	handler := NewHandler(&HandlerOptions{
		Level:        LevelVerbose, // 全局已经开到 verbose
		SampledLevel: LevelDebug,
		Format:       "json",
		Output:       &buf,
	})
	logger := slog.New(handler)

	ctx := trace.ContextWithSpan(context.Background(), &mockSampledSpan{})
	logger.LogAttrs(ctx, LevelVerbose, "verbose in sampled request")

	if buf.Len() == 0 {
		t.Errorf("global=verbose + sampled ctx: verbose log must be emitted, got nothing (sampled branch tightened the level)")
	}
}
