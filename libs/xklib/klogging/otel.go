package klogging

import (
	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// InitDefaultPropagator installs the default header codec (TextMapPropagator)
// into otel's process-global slot. It ONLY selects wire formats — it does not
// create spans or make sampling decisions (that is InitDefaultTracerProvider's
// job). Nothing is parsed until some middleware/client actually calls
// Extract/Inject with this propagator.
//
// Formats accepted on Extract: W3C traceparent/tracestate, B3 single header
// (`b3`, checked first), B3 multi header (`x-b3-*`).
// Formats emitted on Inject: W3C traceparent + B3 multi header.
//
// Note: propagation.Baggage is deliberately NOT registered. Baggage silently
// forwards ctx key-values onto every outgoing request header — a cross-service
// data channel this project has explicitly rejected (KLOG-014, see
// research/2026_0809.CtxInfoRevisit/notes.md). Re-enabling it must be an
// explicit, reviewed change to this function.
func InitDefaultPropagator() {
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{}, // W3C Trace Context
			b3.New(b3.WithInjectEncoding(b3.B3MultipleHeader)), // B3 (Zipkin)
		),
	)
}
