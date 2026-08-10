package klogging

import (
	crand "crypto/rand"
	"context"
	"encoding/binary"
	"io"
	"math/rand"
	"strconv"
	"sync"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

// InitDefaultTracerProvider installs a real (SDK) TracerProvider in no-export
// mode: spans get genuine trace_id/span_id and local sampling decisions, but
// are not sent anywhere (no exporter, no batch worker, no background
// goroutine). This is all that is needed for log/trace correlation.
//
// sampleRatio is the probability that a locally-originated (root) trace is
// sampled (0.0–1.0). Sampled traces log at the Handler's SampledLevel (e.g.
// Debug). Decisions are derived deterministically from the trace_id, so all
// services in a call chain agree. Spans arriving with an upstream decision
// follow it (ParentBased).
//
// All configuration is passed explicitly, which by SDK precedence rules
// disables the OTEL_* environment variable channels (KLOG-005b (3)-3): env
// vars are applied before code options and therefore never override these.
//
// The returned provider's Shutdown should be called on process exit. To later
// export spans to a backend, register a span processor on the returned
// provider — no call-site changes needed.
func InitDefaultTracerProvider(serviceName string, sampleRatio float64) *sdktrace.TracerProvider {
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(sampleRatio))),
		sdktrace.WithIDGenerator(newIDGeneratorFromReader(crand.Reader)),
		// Explicit limits (SDK defaults, minus the env channel). Bounded
		// queues protect memory if some span accumulates events/links.
		sdktrace.WithSpanLimits(sdktrace.SpanLimits{
			AttributeValueLengthLimit:   sdktrace.DefaultAttributeValueLengthLimit,
			AttributeCountLimit:         sdktrace.DefaultAttributeCountLimit,
			EventCountLimit:             sdktrace.DefaultEventCountLimit,
			LinkCountLimit:              sdktrace.DefaultLinkCountLimit,
			AttributePerEventCountLimit: sdktrace.DefaultAttributePerEventCountLimit,
			AttributePerLinkCountLimit:  sdktrace.DefaultAttributePerLinkCountLimit,
		}),
		sdktrace.WithResource(resource.NewSchemaless(semconv.ServiceName(serviceName))),
		// No WithBatcher / WithSpanProcessor: no-export mode.
	)
	otel.SetTracerProvider(tp)
	return tp
}

// ShutdownTracerProvider flushes and stops the given provider, ignoring the
// no-op case. Convenience for main() defer.
func ShutdownTracerProvider(ctx context.Context, tp *sdktrace.TracerProvider) {
	if tp != nil {
		_ = tp.Shutdown(ctx)
	}
}

// ParseSampleRatio parses a sampling ratio string (e.g. "0.01") for
// InitDefaultTracerProvider. Empty or invalid input returns 0 — trace IDs are
// still generated at ratio 0 (log correlation keeps working); only the
// sampled-request debug boost stays off. Same forgiving style as ParseLevel.
func ParseSampleRatio(s string) float64 {
	if s == "" {
		return 0
	}
	ratio, err := strconv.ParseFloat(s, 64)
	if err != nil || ratio < 0 || ratio > 1 {
		return 0
	}
	return ratio
}

// failFastIDGenerator is shape-identical to the SDK's default generator
// (mutex + PRNG stream; the ns-scale critical section is acceptable — see
// KLOG-005a). The one difference: a failed entropy read at seeding time
// panics instead of silently degrading to a fixed seed-0 sequence
// (KLOG-005b (3)-1: multiple instances silently emitting identical ID
// sequences would corrupt cross-pod log correlation with no symptom).
// Seeding happens once at startup — exactly when fail-fast is cheapest.
type failFastIDGenerator struct {
	sync.Mutex
	randSource *rand.Rand
}

var _ sdktrace.IDGenerator = &failFastIDGenerator{}

func newIDGeneratorFromReader(r io.Reader) sdktrace.IDGenerator {
	var seed int64
	if err := binary.Read(r, binary.LittleEndian, &seed); err != nil {
		ke := kerror.Create("TraceIDSeedFailed", "cannot seed trace ID generator from entropy source").
			With("error", err.Error())
		panic(ke)
	}
	return &failFastIDGenerator{randSource: rand.New(rand.NewSource(seed))}
}

// NewIDs returns a new trace and span ID.
func (gen *failFastIDGenerator) NewIDs(ctx context.Context) (trace.TraceID, trace.SpanID) {
	gen.Lock()
	defer gen.Unlock()
	tid := trace.TraceID{}
	_, _ = gen.randSource.Read(tid[:])
	sid := trace.SpanID{}
	_, _ = gen.randSource.Read(sid[:])
	return tid, sid
}

// NewSpanID returns a span ID for an existing trace.
func (gen *failFastIDGenerator) NewSpanID(ctx context.Context, traceID trace.TraceID) trace.SpanID {
	gen.Lock()
	defer gen.Unlock()
	sid := trace.SpanID{}
	_, _ = gen.randSource.Read(sid[:])
	return sid
}
