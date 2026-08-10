package klogging

import (
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// TracingMiddleware makes each HTTP request trace-aware:
//
//  1. Extract: parses incoming trace headers (W3C traceparent / B3, per the
//     propagator installed by InitDefaultPropagator) into the request ctx.
//  2. Start: opens a server span — continuing the upstream trace if one was
//     extracted, or starting a new root trace otherwise (sampling decided by
//     the provider installed by InitDefaultTracerProvider).
//
// Every log written via slog.XxxContext(r.Context(), ...) inside the request
// then carries trace_id/span_id automatically.
//
// This is a plain primitive: wrap whichever mux/handler needs it, compose
// with other middlewares in any order. Without the two Init calls it degrades
// to a no-op passthrough (noop tracer, empty propagator).
func TracingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		ctx, span := otel.Tracer("xklib/klogging").Start(ctx, r.Method+" "+r.URL.Path,
			trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
