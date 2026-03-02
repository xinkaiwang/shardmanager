package klogging

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

func BenchmarkHandler(b *testing.B) {
	handler := NewHandler(&HandlerOptions{
		Level:  slog.LevelInfo,
		Format: "json",
		Output: io.Discard,
	})
	logger := slog.New(handler)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		logger.InfoContext(ctx, "benchmark message",
			slog.String("event", "BenchEvent"),
			slog.Int("count", i),
		)
	}
}

func BenchmarkHandler_WithMetrics(b *testing.B) {
	reporter := &mockMetricsReporter{}
	handler := NewHandler(&HandlerOptions{
		Level:   slog.LevelInfo,
		Format:  "json",
		Output:  io.Discard,
		Metrics: reporter,
	})
	logger := slog.New(handler)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		logger.InfoContext(ctx, "benchmark message",
			slog.String("event", "BenchEvent"),
			slog.Int("count", i),
		)
	}
}

func BenchmarkHandler_Sampling(b *testing.B) {
	handler := NewHandler(&HandlerOptions{
		Level:        slog.LevelInfo,
		SampledLevel: slog.LevelDebug,
		Format:       "json",
		Output:       io.Discard,
	})
	logger := slog.New(handler)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		logger.DebugContext(ctx, "benchmark message",
			slog.String("event", "BenchEvent"),
			slog.Int("count", i),
		)
	}
}
