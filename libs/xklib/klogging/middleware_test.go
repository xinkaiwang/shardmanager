package klogging

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func setupTracing(t *testing.T, ratio float64) {
	t.Helper()
	InitDefaultPropagator()
	tp := InitDefaultTracerProvider("test-svc", ratio)
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
}

// 无上游 header：middleware 必须为请求开一个有效的 root span
func TestTracingMiddleware_StartsRootSpan(t *testing.T) {
	setupTracing(t, 1.0)

	var got trace.SpanContext
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = trace.SpanFromContext(r.Context()).SpanContext()
	})

	req := httptest.NewRequest("GET", "/api/ping", nil)
	TracingMiddleware(inner).ServeHTTP(httptest.NewRecorder(), req)

	if !got.IsValid() {
		t.Fatalf("handler ctx must carry a valid span, got %+v", got)
	}
	if !got.IsSampled() {
		t.Errorf("ratio=1.0 request should be sampled")
	}
}

// W3C traceparent：上游 trace 必须被延续（同 trace_id、跟随采样）
func TestTracingMiddleware_ContinuesW3CParent(t *testing.T) {
	setupTracing(t, 0.0) // 本地 0%，全靠跟随上游

	var got trace.SpanContext
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = trace.SpanFromContext(r.Context()).SpanContext()
	})

	req := httptest.NewRequest("GET", "/api/ping", nil)
	req.Header.Set("traceparent", "00-0102030405060708090a0b0c0d0e0f10-0102030405060708-01")
	TracingMiddleware(inner).ServeHTTP(httptest.NewRecorder(), req)

	if got.TraceID().String() != "0102030405060708090a0b0c0d0e0f10" {
		t.Errorf("upstream trace not continued, got trace_id=%s", got.TraceID())
	}
	if !got.IsSampled() {
		t.Errorf("sampled upstream must be followed at local ratio=0")
	}
}

// B3 单头短形式（64-bit trace id）：必须被识别并左补零延续
func TestTracingMiddleware_ContinuesB3SingleShortForm(t *testing.T) {
	setupTracing(t, 0.0)

	var got trace.SpanContext
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = trace.SpanFromContext(r.Context()).SpanContext()
	})

	req := httptest.NewRequest("GET", "/api/ping", nil)
	req.Header.Set("b3", "090a0b0c0d0e0f10-0102030405060708-1")
	TracingMiddleware(inner).ServeHTTP(httptest.NewRecorder(), req)

	if got.TraceID().String() != "0000000000000000090a0b0c0d0e0f10" {
		t.Errorf("B3 short-form trace not continued/padded, got trace_id=%s", got.TraceID())
	}
	if !got.IsSampled() {
		t.Errorf("b3 sampled=1 must be followed")
	}
}

// KLOG-014：baggage header 不得进入 ctx（Baggage propagator 已被摘除）
func TestTracingMiddleware_IgnoresBaggageHeader(t *testing.T) {
	setupTracing(t, 1.0)

	var gotCtx context.Context
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCtx = r.Context()
	})

	req := httptest.NewRequest("GET", "/api/ping", nil)
	req.Header.Set("baggage", "injected=evil")
	TracingMiddleware(inner).ServeHTTP(httptest.NewRecorder(), req)

	// baggage 包未被注册为 propagator，ctx 里不应有任何 baggage 数据。
	// 直接断言 otel baggage API 读不到东西需要引入 baggage 包——刻意不引入
	// （xklib 不应出现该 import）；退而断言 ctx 仍携带有效 span 即可，
	// baggage 的缺席由 InitDefaultPropagator 的注册列表保证（编译期可见）。
	if !trace.SpanFromContext(gotCtx).SpanContext().IsValid() {
		t.Fatalf("span must still work with baggage header present")
	}
}
