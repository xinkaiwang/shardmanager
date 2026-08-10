// oteldemo: 演示 OTel SDK (no-export 模式) 对日志 trace 关联的效果。
// 对应 research/2026_0809.CtxInfoRevisit/ 的 D1 决策演示。
//
// 三个场景：
//  1. 不装 SDK（当前生产现状）—— 日志没有任何 trace_id
//  2. 装 SDK + 50% 本地采样 —— 每个"请求"有真实 trace_id；被采样的请求自动打出 Debug 日志
//  3. 跨异步边界（模拟 runloop PostEvent）—— 后台处理有自己的 trace_id，并用 span link 指回投递方
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func main() {
	ctx := context.Background()

	// klogging 初始化（和生产完全一致）
	klogging.InitOpenTelemetry()
	handler := klogging.NewHandler(&klogging.HandlerOptions{
		Level:        klogging.LevelInfo,  // 全局级别: Info（Debug 默认被压掉）
		SampledLevel: klogging.LevelDebug, // 被采样的请求: 降到 Debug
		Format:       "json",
		Output:       os.Stdout,
	})
	slog.SetDefault(slog.New(handler))

	fmt.Println("========== 场景 1: 不装 SDK（当前生产现状） ==========")
	scenario1(ctx)

	// ---- 安装 SDK：no-export 模式 ----
	// 注意：没有 WithBatcher / WithSpanProcessor —— span 不上报任何后端，
	// 仅本地生成 trace_id/span_id + 执行采样决策。
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(0.5))), // 50% 采样，便于演示
	)
	otel.SetTracerProvider(tp)
	defer func() { _ = tp.Shutdown(ctx) }()

	fmt.Println()
	fmt.Println("========== 场景 2: 装 SDK 后，模拟 6 个 HTTP 请求（50% 本地采样） ==========")
	for i := 1; i <= 6; i++ {
		handleRequest(ctx, i)
	}

	fmt.Println()
	fmt.Println("========== 场景 3: 跨异步边界（模拟 runloop PostEvent, 100% 采样） ==========")
	tpAlways := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tpAlways)
	defer func() { _ = tpAlways.Shutdown(ctx) }()
	scenario3(ctx)
}

// 场景 1：没有 SDK 时，tracer.Start 返回 noop span，日志没有 trace_id
func scenario1(ctx context.Context) {
	tracer := otel.Tracer("demo")
	ctx2, span := tracer.Start(ctx, "HTTP GET /api/foo") // noop!
	defer span.End()

	fmt.Printf("--> span 有效吗? %v (noop span, 没有 SDK 就造不出 ID)\n", span.SpanContext().IsValid())
	slog.InfoContext(ctx2, "handling request",
		slog.String("event", "RequestStart"))
	slog.DebugContext(ctx2, "detail info",
		slog.String("event", "RequestDetail")) // 被压掉：无采样概念
}

// 场景 2：每个请求开一个 root span，采样决策由本地 sampler 做
func handleRequest(ctx context.Context, reqId int) {
	tracer := otel.Tracer("demo")
	ctx2, span := tracer.Start(ctx, "HTTP GET /api/foo")
	defer span.End()

	sampled := span.SpanContext().IsSampled()
	fmt.Printf("--> 请求 #%d  sampled=%v\n", reqId, sampled)

	// Info 级别：所有请求都打出，且自动带 trace_id/span_id
	slog.InfoContext(ctx2, "handling request",
		slog.String("event", "RequestStart"),
		slog.Int("reqId", reqId))

	// Debug 级别：只有被采样的请求打出（Handler.Enabled 动态提级）
	slog.DebugContext(ctx2, "expensive debug detail",
		slog.String("event", "RequestDetail"),
		slog.Int("reqId", reqId))
}

// demoEvent 模拟 krunloop.IEvent：创建时捕获投递方的 SpanContext（一个值拷贝）
type demoEvent struct {
	name   string
	origin trace.SpanContext
}

// 场景 3：请求 → PostEvent → runloop 后台处理
func scenario3(ctx context.Context) {
	tracer := otel.Tracer("demo")

	// (a) 请求侧：处理 HTTP 请求，投递一个事件
	reqCtx, reqSpan := tracer.Start(ctx, "HTTP POST /api/move-shard")
	slog.InfoContext(reqCtx, "request received, posting event to runloop",
		slog.String("event", "MoveShardRequest"))

	eve := demoEvent{
		name:   "MoveShardEvent",
		origin: trace.SpanContextFromContext(reqCtx), // ← PostEvent 时捕获
	}
	reqSpan.End()
	// （HTTP 响应已返回，请求 ctx 的生命周期结束）

	// (b) runloop 侧：另一个 goroutine、全新的 ctx，用 span link 接回来源
	bgCtx := context.Background()
	linkOpt := trace.WithLinks(trace.Link{SpanContext: eve.origin})
	evtCtx, evtSpan := tracer.Start(bgCtx, "runloop:"+eve.name, linkOpt)
	defer evtSpan.End()

	slog.InfoContext(evtCtx, "processing event in runloop",
		slog.String("event", "MoveShardProcessing"),
		slog.String("linkedOriginTraceId", eve.origin.TraceID().String())) // 演示用：展示 link 指向
	slog.InfoContext(evtCtx, "event done",
		slog.String("event", "MoveShardDone"))

	fmt.Printf("--> 请求侧 trace_id   = %s\n", eve.origin.TraceID())
	fmt.Printf("--> 后台处理 trace_id = %s (自己的链路, 经 span link 可追回请求)\n",
		evtSpan.SpanContext().TraceID())
}
