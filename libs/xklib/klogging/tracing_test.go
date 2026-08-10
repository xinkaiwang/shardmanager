package klogging

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

// ratio=1.0：root span 必须有有效 trace_id/span_id 且被采样
func TestInitDefaultTracerProvider_GeneratesValidSampledIDs(t *testing.T) {
	tp := InitDefaultTracerProvider("test-svc", 1.0)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	_, span := tp.Tracer("test").Start(context.Background(), "op")
	defer span.End()

	sc := span.SpanContext()
	if !sc.IsValid() {
		t.Fatalf("expected valid span context, got %+v", sc)
	}
	if !sc.IsSampled() {
		t.Errorf("ratio=1.0 root span should be sampled")
	}
}

// ratio=0：span 未采样，但 trace_id 依然有效（日志关联 100% 覆盖的关键性质）
func TestInitDefaultTracerProvider_RatioZeroStillHasValidIDs(t *testing.T) {
	tp := InitDefaultTracerProvider("test-svc", 0.0)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	_, span := tp.Tracer("test").Start(context.Background(), "op")
	defer span.End()

	sc := span.SpanContext()
	if !sc.IsValid() {
		t.Fatalf("unsampled span must still carry valid IDs, got %+v", sc)
	}
	if sc.IsSampled() {
		t.Errorf("ratio=0 root span should not be sampled")
	}
}

// ParentBased：上游已采样的 remote parent，子 span 跟随采样决策且延续 trace_id
func TestInitDefaultTracerProvider_FollowsRemoteParent(t *testing.T) {
	tp := InitDefaultTracerProvider("test-svc", 0.0) // 本地 0%，全靠跟随
	defer func() { _ = tp.Shutdown(context.Background()) }()

	parentSc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanID:     trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8},
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})
	ctx := trace.ContextWithRemoteSpanContext(context.Background(), parentSc)

	_, span := tp.Tracer("test").Start(ctx, "op")
	defer span.End()

	sc := span.SpanContext()
	if sc.TraceID() != parentSc.TraceID() {
		t.Errorf("child must continue parent trace: got %s want %s", sc.TraceID(), parentSc.TraceID())
	}
	if !sc.IsSampled() {
		t.Errorf("sampled remote parent must be followed even at local ratio=0")
	}
}

// KLOG-005b ③-3：环境变量不得覆盖代码显式配置
func TestInitDefaultTracerProvider_EnvVarDoesNotOverrideExplicitSampler(t *testing.T) {
	t.Setenv("OTEL_TRACES_SAMPLER", "always_off")

	tp := InitDefaultTracerProvider("test-svc", 1.0)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	_, span := tp.Tracer("test").Start(context.Background(), "op")
	defer span.End()

	if !span.SpanContext().IsSampled() {
		t.Errorf("explicit ratio=1.0 must win over OTEL_TRACES_SAMPLER=always_off")
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("no entropy") }

// KLOG-005b ③-1：种子读取失败必须响亮地死（fail-fast），不得静默降级为固定序列
func TestIDGenerator_PanicsOnSeedFailure(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on seed failure, got none")
		}
	}()
	newIDGeneratorFromReader(failingReader{})
}
