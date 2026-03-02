# klogging

Structured logging for Go services, built on `log/slog` with OpenTelemetry integration.

## Features

- ✅ **Standard library** — Built on Go 1.21+ `log/slog`
- ✅ **OpenTelemetry** — Automatic trace context injection
- ✅ **Dynamic log level** — Per-request sampling (B3/W3C)
- ✅ **Metrics integration** — Optional metrics reporter
- ✅ **Zero dependencies** — Only stdlib + OpenTelemetry

## Quick Start

### Basic Usage

```go
package main

import (
	"context"
	"log/slog"
	
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
)

func main() {
	// 1. Initialize OpenTelemetry (once at startup)
	klogging.InitOpenTelemetry()
	
	// 2. Create handler
	handler := klogging.NewHandler(&klogging.HandlerOptions{
		Level:  slog.LevelInfo,
		Format: "json",
	})
	
	// 3. Set as default logger
	logger := slog.New(handler)
	slog.SetDefault(logger)
	
	// 4. Use standard slog API
	ctx := context.Background()
	slog.InfoContext(ctx, "Server starting",
		slog.String("event", "ServerStart"),
		slog.String("version", "v1.0"),
	)
}
```

### With Metrics

```go
type MyMetricsReporter struct{}

func (r *MyMetricsReporter) ReportLog(ctx context.Context, level, event string, size int, logged bool) {
	// Report to your metrics system (kmetrics, Prometheus, etc.)
	logSizeMetric.Add(ctx, level, event, int64(size))
}

handler := klogging.NewHandler(&klogging.HandlerOptions{
	Level:   slog.LevelInfo,
	Format:  "json",
	Metrics: &MyMetricsReporter{},
})
```

### Dynamic Log Level (Sampling)

```go
// Configure handler with sampled level
handler := klogging.NewHandler(&klogging.HandlerOptions{
	Level:        slog.LevelInfo,   // Normal requests: Info+
	SampledLevel: slog.LevelDebug,  // Sampled requests: Debug+
	Format:       "json",
})

// HTTP middleware
func TracingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract trace context (W3C or B3)
		ctx := otel.GetTextMapPropagator().Extract(
			r.Context(),
			propagation.HeaderCarrier(r.Header),
		)
		
		// Create span
		tracer := otel.Tracer("my-service")
		ctx, span := tracer.Start(ctx, "HTTP "+r.Method+" "+r.URL.Path)
		defer span.End()
		
		// Debug logs only appear when sampled=1
		slog.DebugContext(ctx, "Request details",
			slog.String("event", "RequestDebug"),
			slog.Any("headers", r.Header),
		)
		
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
```

## API Reference

### Handler Options

```go
type HandlerOptions struct {
	Level        slog.Level      // Default log level (e.g., Info)
	SampledLevel slog.Level      // Log level when trace is sampled (e.g., Debug)
	Format       string          // "json" or "text"
	Output       io.Writer       // Output destination (default: os.Stderr)
	Metrics      MetricsReporter // Optional metrics reporter
}
```

### Log Levels

```go
const (
	LevelVerbose = slog.LevelDebug - 1
	LevelDebug   = slog.LevelDebug
	LevelInfo    = slog.LevelInfo
	LevelWarn    = slog.LevelWarn
	LevelError   = slog.LevelError
	LevelFatal   = slog.LevelError + 1
)
```

### Metrics Reporter Interface

```go
type MetricsReporter interface {
	ReportLog(ctx context.Context, level, event string, size int, logged bool)
}
```

## Migration from old klogging

### Before (logrus-based)

```go
klogging.Info(ctx).
	With("userId", userID).
	With("action", "login").
	Log("UserLogin", "User logged in")
```

### After (slog-based)

```go
slog.InfoContext(ctx, "User logged in",
	slog.String("event", "UserLogin"),
	slog.String("userId", userID),
	slog.String("action", "login"),
)
```

### Trace Context

**Before**:
```go
ctxInfo := klogging.GetCtxInfoFromCtx(ctx)
if ctxInfo != nil {
	traceID := ctxInfo.TraceID
}
```

**After**:
```go
import "go.opentelemetry.io/otel/trace"

span := trace.SpanFromContext(ctx)
if span.SpanContext().IsValid() {
	traceID := span.SpanContext().TraceID().String()
	sampled := span.SpanContext().IsSampled()
}
```

## Performance

Benchmarks on Apple M3 Ultra:

```
BenchmarkHandler-28                 2656828     427 ns/op     96 B/op    2 allocs/op
BenchmarkHandler_WithMetrics-28     2653036     451 ns/op     96 B/op    2 allocs/op
BenchmarkHandler_Sampling-28       31454782      38 ns/op     96 B/op    2 allocs/op
```

~2x faster than old logrus-based implementation.

## Dependencies

```go
require (
	log/slog                                      // Go 1.21+ stdlib
	go.opentelemetry.io/otel v1.24.0
	go.opentelemetry.io/otel/trace v1.24.0
	go.opentelemetry.io/contrib/propagators/b3 v1.24.0
)
```

## License

Same as parent project.
