package klogging

import (
	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// InitOpenTelemetry configures OpenTelemetry propagators.
// Supports W3C Trace Context (standard) and B3 (Zipkin/Brave) formats.
// Call this once during application startup.
func InitOpenTelemetry() {
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{}, // W3C Trace Context (recommended)
			propagation.Baggage{},      // W3C Baggage
			b3.New(b3.WithInjectEncoding(b3.B3MultipleHeader)), // B3 (Zipkin compatibility)
		),
	)
}
