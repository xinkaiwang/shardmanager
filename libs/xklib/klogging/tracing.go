package klogging

import (
	"context"
	"math/rand"
	"strconv"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
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
	// Warm-up draw: kcommon's PRNG seeds lazily and fail-fasts on a dead
	// entropy source (KLOG-013). Forcing the seeding here moves that
	// potential panic to startup — the cheapest possible time to die —
	// instead of the first span of some request.
	kcommon.GetRandom(context.Background(), func(*rand.Rand) {})

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(sampleRatio))),
		sdktrace.WithIDGenerator(kcommonIDGenerator{}),
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

// kcommonIDGenerator delegates ID generation to kcommon's SafeRand — the
// project's single random primitive (one PRNG to audit instead of two
// parallel crypto-seeded streams). Semantics match the D1 decision: mutex +
// crypto-seeded PRNG (ns-scale critical section, acceptable per KLOG-005a),
// fail-fast on entropy failure (kcommon panics with kerror; the seed-failure
// path is tested where the code lives, kcommon/rand_util_test.go). Trace IDs
// need uniqueness, not unpredictability (KLOG-013①) — sharing the stream
// with jitter consumers does not weaken either.
type kcommonIDGenerator struct{}

var _ sdktrace.IDGenerator = kcommonIDGenerator{}

// NewIDs returns a new trace and span ID.
func (kcommonIDGenerator) NewIDs(ctx context.Context) (trace.TraceID, trace.SpanID) {
	tid := trace.TraceID{}
	sid := trace.SpanID{}
	kcommon.GetRandom(ctx, func(r *rand.Rand) {
		_, _ = r.Read(tid[:])
		_, _ = r.Read(sid[:])
	})
	return tid, sid
}

// NewSpanID returns a span ID for an existing trace.
func (kcommonIDGenerator) NewSpanID(ctx context.Context, traceID trace.TraceID) trace.SpanID {
	sid := trace.SpanID{}
	kcommon.GetRandom(ctx, func(r *rand.Rand) {
		_, _ = r.Read(sid[:])
	})
	return sid
}
