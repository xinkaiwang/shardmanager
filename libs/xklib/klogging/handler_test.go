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

	if reporter.callCount == 0 {
		t.Error("Expected metrics to be reported")
	}
	if reporter.lastEvent != "TestEvent" {
		t.Errorf("Expected event=TestEvent, got %s", reporter.lastEvent)
	}
	if reporter.lastLevel != "INFO" {
		t.Errorf("Expected level=INFO, got %s", reporter.lastLevel)
	}
	if !reporter.lastLogged {
		t.Error("Expected logged=true")
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

type mockMetricsReporter struct {
	callCount  int
	lastLevel  string
	lastEvent  string
	lastSize   int
	lastLogged bool
}

func (r *mockMetricsReporter) ReportLog(ctx context.Context, level, event string, size int, logged bool) {
	r.callCount++
	r.lastLevel = level
	r.lastEvent = event
	r.lastSize = size
	r.lastLogged = logged
}
