package klogging

import (
	"context"
	"io"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/trace"
)

// Handler wraps slog.Handler with klogging-specific features:
// - Metrics integration
// - Dynamic log level based on OpenTelemetry sampling
// - Automatic trace context injection
type Handler struct {
	baseHandler  slog.Handler
	globalLevel  slog.Level
	sampledLevel slog.Level
	metrics      MetricsReporter
}

// MetricsReporter reports logging metrics (optional).
type MetricsReporter interface {
	ReportLog(ctx context.Context, level, event string, size int, logged bool)
}

// HandlerOptions configures a Handler.
type HandlerOptions struct {
	Level        slog.Level      // Default log level (e.g., Info)
	SampledLevel slog.Level      // Log level when trace is sampled (e.g., Debug)
	Format       string          // "json" or "text"
	Output       io.Writer       // Output destination (default: os.Stderr)
	Metrics      MetricsReporter // Optional metrics reporter
}

// NewHandler creates a new klogging Handler.
func NewHandler(opts *HandlerOptions) *Handler {
	if opts == nil {
		opts = &HandlerOptions{}
	}
	
	// Defaults
	if opts.Level == 0 {
		opts.Level = slog.LevelInfo
	}
	if opts.SampledLevel == 0 {
		opts.SampledLevel = slog.LevelDebug
	}
	if opts.Output == nil {
		opts.Output = os.Stderr
	}
	if opts.Format == "" {
		opts.Format = "json"
	}
	
	// Create base handler
	var baseHandler slog.Handler
	handlerOpts := &slog.HandlerOptions{
		Level: opts.SampledLevel, // Set to lowest level, Enabled() controls filtering
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Format timestamps as RFC3339 with millisecond precision (3 decimal digits).
			// Default slog uses RFC3339Nano (6 digits) which is unnecessarily verbose.
			if a.Key == slog.TimeKey && a.Value.Kind() == slog.KindTime {
				return slog.String(a.Key, a.Value.Time().Format("2006-01-02T15:04:05.000Z07:00"))
			}
			return a
		},
	}
	
	switch opts.Format {
	case "json":
		baseHandler = slog.NewJSONHandler(opts.Output, handlerOpts)
	case "text":
		baseHandler = slog.NewTextHandler(opts.Output, handlerOpts)
	default:
		baseHandler = slog.NewJSONHandler(opts.Output, handlerOpts)
	}
	
	return &Handler{
		baseHandler:  baseHandler,
		globalLevel:  opts.Level,
		sampledLevel: opts.SampledLevel,
		metrics:      opts.Metrics,
	}
}

// Enabled implements slog.Handler.
// Dynamically determines if a log should be emitted based on:
// 1. OpenTelemetry sampling (sampled → use sampledLevel)
// 2. Default global level
func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	// Check OpenTelemetry sampling
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() && span.SpanContext().IsSampled() {
		return level >= h.sampledLevel
	}
	
	// Default level
	return level >= h.globalLevel
}

// Handle implements slog.Handler.
// Injects trace context and reports metrics before delegating to base handler.
func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	// Inject OpenTelemetry trace context
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		sc := span.SpanContext()
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	
	// Report metrics
	if h.metrics != nil {
		event, size := extractEventAndSize(r)
		logged := h.Enabled(ctx, r.Level)
		h.metrics.ReportLog(ctx, r.Level.String(), event, size, logged)
	}
	
	// Delegate to base handler
	return h.baseHandler.Handle(ctx, r)
}

// WithAttrs implements slog.Handler.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{
		baseHandler:  h.baseHandler.WithAttrs(attrs),
		globalLevel:  h.globalLevel,
		sampledLevel: h.sampledLevel,
		metrics:      h.metrics,
	}
}

// WithGroup implements slog.Handler.
func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{
		baseHandler:  h.baseHandler.WithGroup(name),
		globalLevel:  h.globalLevel,
		sampledLevel: h.sampledLevel,
		metrics:      h.metrics,
	}
}

// extractEventAndSize extracts the "event" field and estimates log size.
func extractEventAndSize(r slog.Record) (event string, size int) {
	size = len(r.Message)
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "event" {
			event = a.Value.String()
		}
		size += len(a.Key) + estimateAttrSize(a.Value)
		return true
	})
	return
}

// estimateAttrSize estimates the byte size of a slog.Value.
func estimateAttrSize(v slog.Value) int {
	switch v.Kind() {
	case slog.KindString:
		return len(v.String())
	case slog.KindInt64:
		return 8
	case slog.KindUint64:
		return 8
	case slog.KindFloat64:
		return 8
	case slog.KindBool:
		return 1
	case slog.KindDuration:
		return 8
	case slog.KindTime:
		return 32
	default:
		return len(v.String())
	}
}
