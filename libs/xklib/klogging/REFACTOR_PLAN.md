# klogging 重构计划：迁移到 slog + OpenTelemetry

**作者**: 大白 (Bruce)  
**日期**: 2026-03-01  
**版本**: v1.0  

---

## 执行摘要

将 klogging 从 **logrus + 自定义 CtxInfo** 迁移到 **log/slog + OpenTelemetry**。

**关键收益**：
- ✅ **性能提升 5x** — slog zero-allocation paths
- ✅ **零外部依赖** — 移除 logrus（标准库）
- ✅ **行业标准** — OpenTelemetry CNCF 项目
- ✅ **生态兼容** — 所有 APM 系统（Jaeger/Zipkin/Datadog）
- ✅ **长期维护** — Go team 官方支持

**迁移周期**：4-6 周（渐进式，零破坏）

---

## 目录

1. [当前架构分析](#1-当前架构分析)
2. [目标架构](#2-目标架构)
3. [迁移策略](#3-迁移策略)
4. [实施计划](#4-实施计划)
5. [代码示例](#5-代码示例)
6. [测试策略](#6-测试策略)
7. [风险评估](#7-风险评估)
8. [回滚计划](#8-回滚计划)

---

## 1. 当前架构分析

### 1.1 当前技术栈

```
应用代码
  ↓
klogging.Info(ctx).With("x", y).Log("event", "msg")
  ↓
nlogging.go (API 层)
  ↓
LogrusLogger (实现层) + CtxInfo (自定义 trace)
  ↓
logrus (第三方库)
  ↓
os.Stderr / file
```

### 1.2 核心组件

| 组件 | 文件 | 职责 | 问题 |
|------|------|------|------|
| `nlogging.go` | API 层 | 链式 builder API | 性能：每次 log 都 allocate |
| `logrus_logger.go` | 实现层 | Metrics 集成 | 依赖 logrus（第三方） |
| `ctx_info.go` | Context 传播 | 手动 trace propagation | 孤立，不兼容 OpenTelemetry |
| `simple_formatter.go` | 格式化 | 自定义格式 | Map iteration 不稳定 |

### 1.3 核心功能

✅ **必须保留的功能**：
1. Event-driven logging（`event` 字段约定）
2. Metrics 集成（自动统计日志量）
3. B3 采样（`sampled=1` 提升日志级别）
4. 链式 API（`Info(ctx).With(k, v).Log()`）
5. 多格式支持（JSON/Text/Simple）

❌ **需要改进的部分**：
1. 性能（~1000ns/op → ~200ns/op）
2. Trace propagation（手动 → 自动）
3. 生态兼容（孤立 → OpenTelemetry）
4. 维护成本（自己维护 → Go team）

---

## 2. 目标架构

### 2.1 新技术栈

```
应用代码
  ↓
slog.Info(ctx, "msg", slog.String("event", "..."))
  ↓
slog.Logger (标准库)
  ↓
KloggingHandler (implements slog.Handler)
  ├─ Metrics 集成
  ├─ OpenTelemetry trace injection
  └─ 委托给 JSONHandler/TextHandler
  ↓
os.Stderr / file
```

### 2.2 组件对比

| 功能 | Before (current) | After (target) |
|------|------------------|----------------|
| **日志框架** | logrus (第三方) | slog (标准库) |
| **API 风格** | 链式 builder | slog 标准 + 兼容层 |
| **Trace 传播** | 自定义 CtxInfo | OpenTelemetry |
| **Metrics** | LoggerMetricsReporter | 保留 |
| **格式化** | logrus formatters | slog.Handler |
| **性能** | ~1000ns/op | ~200ns/op |
| **外部依赖** | logrus, sirupsen | 零依赖（仅标准库） |

### 2.3 目标依赖

```go
// 核心（零额外依赖）
import "log/slog"

// OpenTelemetry（行业标准）
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/trace"
    "go.opentelemetry.io/contrib/propagators/b3"  // B3 兼容
)
```

---

## 3. 迁移策略

### 3.1 核心原则

1. **渐进式** — 不做 big bang 重写，逐步迁移
2. **零破坏** — 保持 API 兼容（wrapper 模式）
3. **可回滚** — 每个 phase 都可独立回滚
4. **可验证** — 每个 phase 都有测试验证

### 3.2 三阶段迁移

```
Phase 1: Foundation (1-2 周)
├─ 实现 KloggingHandler (slog.Handler)
├─ 添加 OpenTelemetry
├─ 保留 CtxInfo（兼容层）
└─ 单元测试 + benchmark

Phase 2: Gradual Migration (2-3 周)
├─ Middleware 同时支持两种
├─ 新代码用 slog + OpenTelemetry
├─ 旧代码继续用 klogging API（wrapper）
└─ 集成测试

Phase 3: Cleanup (1 周)
├─ 删除 logrus 依赖
├─ 删除 CtxInfo
├─ 删除兼容层（可选）
└─ 性能测试
```

---

## 4. 实施计划

### Phase 1: Foundation (Week 1-2)

**目标**：实现新架构，保持完全兼容

#### 4.1.1 创建 KloggingHandler

**新文件**: `libs/xklib/klogging/slog_handler.go`

```go
package klogging

import (
    "context"
    "log/slog"
    "os"
    "go.opentelemetry.io/otel/trace"
)

type SlogHandler struct {
    baseHandler  slog.Handler
    globalLevel  slog.Level
    sampledLevel slog.Level
    metrics      LoggerMetricsReporter
}

type SlogHandlerOptions struct {
    Level           slog.Level  // 默认 slog.LevelInfo
    SampledLevel    slog.Level  // 采样时 slog.LevelDebug
    Format          string      // "json", "text"
    Output          io.Writer   // os.Stderr, file, etc.
    MetricsReporter LoggerMetricsReporter
}

func NewSlogHandler(opts *SlogHandlerOptions) *SlogHandler {
    var baseHandler slog.Handler
    
    handlerOpts := &slog.HandlerOptions{
        Level: opts.SampledLevel, // 设为最低级别，由 Enabled() 控制
    }
    
    switch opts.Format {
    case "json":
        baseHandler = slog.NewJSONHandler(opts.Output, handlerOpts)
    case "text":
        baseHandler = slog.NewTextHandler(opts.Output, handlerOpts)
    default:
        baseHandler = slog.NewJSONHandler(opts.Output, handlerOpts)
    }
    
    return &SlogHandler{
        baseHandler:  baseHandler,
        globalLevel:  opts.Level,
        sampledLevel: opts.SampledLevel,
        metrics:      opts.MetricsReporter,
    }
}

// 实现 slog.Handler 接口
func (h *SlogHandler) Enabled(ctx context.Context, level slog.Level) bool {
    // 动态级别判断（支持 OpenTelemetry + 兼容 CtxInfo）
    
    // 1. 优先使用 OpenTelemetry
    if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
        if span.SpanContext().IsSampled() {
            return level >= h.sampledLevel
        }
    }
    
    // 2. 兼容旧的 CtxInfo（过渡期）
    if ctxInfo := GetCtxInfoFromCtx(ctx); ctxInfo != nil {
        if ctxInfo.Sampled == "1" {
            return level >= h.sampledLevel
        }
    }
    
    // 3. 默认全局级别
    return level >= h.globalLevel
}

func (h *SlogHandler) Handle(ctx context.Context, r slog.Record) error {
    // 1. 注入 OpenTelemetry trace context
    if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
        sc := span.SpanContext()
        r.Add(
            slog.String("trace_id", sc.TraceID().String()),
            slog.String("span_id", sc.SpanID().String()),
        )
    } else if ctxInfo := GetCtxInfoFromCtx(ctx); ctxInfo != nil {
        // 兼容旧 CtxInfo（过渡期）
        r.Add(
            slog.String("trace_id", ctxInfo.TraceID),
            slog.String("span_id", ctxInfo.SpanID),
        )
    }
    
    // 2. Metrics 集成
    if h.metrics != nil {
        size := len(r.Message)
        event := ""
        r.Attrs(func(a slog.Attr) bool {
            if a.Key == "event" {
                event = a.Value.String()
            }
            size += len(a.Key) + estimateAttrSize(a.Value)
            return true
        })
        
        shouldLog := h.Enabled(ctx, r.Level)
        h.metrics.ReportLogSizeBytes(ctx, size, r.Level.String(), event)
        if r.Level >= LevelDebug {
            h.metrics.ReportLogErrorCount(ctx, 1, r.Level.String(), event, shouldLog)
        }
    }
    
    // 3. 委托给底层 handler
    return h.baseHandler.Handle(ctx, r)
}

func (h *SlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
    return &SlogHandler{
        baseHandler:  h.baseHandler.WithAttrs(attrs),
        globalLevel:  h.globalLevel,
        sampledLevel: h.sampledLevel,
        metrics:      h.metrics,
    }
}

func (h *SlogHandler) WithGroup(name string) slog.Handler {
    return &SlogHandler{
        baseHandler:  h.baseHandler.WithGroup(name),
        globalLevel:  h.globalLevel,
        sampledLevel: h.sampledLevel,
        metrics:      h.metrics,
    }
}

func estimateAttrSize(v slog.Value) int {
    switch v.Kind() {
    case slog.KindString:
        return len(v.String())
    case slog.KindInt64:
        return 8
    case slog.KindFloat64:
        return 8
    case slog.KindBool:
        return 1
    default:
        return len(v.String())
    }
}
```

#### 4.1.2 添加 OpenTelemetry 支持

**新文件**: `libs/xklib/klogging/otel.go`

```go
package klogging

import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/propagation"
    "go.opentelemetry.io/contrib/propagators/b3"
)

// InitOpenTelemetry 初始化 OpenTelemetry propagator
// 支持 W3C Trace Context 和 B3（Zipkin）格式
func InitOpenTelemetry() {
    otel.SetTextMapPropagator(
        propagation.NewCompositeTextMapPropagator(
            propagation.TraceContext{},  // W3C Trace Context (推荐)
            propagation.Baggage{},       // W3C Baggage
            b3.New(b3.WithInjectEncoding(b3.B3MultipleHeader)), // B3 兼容
        ),
    )
}
```

#### 4.1.3 兼容层（保留旧 API）

**新文件**: `libs/xklib/klogging/compat.go`

```go
package klogging

import (
    "context"
    "log/slog"
)

// 兼容旧的链式 API
func Info(ctx context.Context) *LogEntryBuilder {
    return &LogEntryBuilder{
        ctx:   ctx,
        level: slog.LevelInfo,
        attrs: make([]slog.Attr, 0, 8),
    }
}

func Debug(ctx context.Context) *LogEntryBuilder {
    return &LogEntryBuilder{ctx: ctx, level: slog.LevelDebug, attrs: make([]slog.Attr, 0, 8)}
}

func Verbose(ctx context.Context) *LogEntryBuilder {
    return &LogEntryBuilder{ctx: ctx, level: slog.LevelDebug - 1, attrs: make([]slog.Attr, 0, 8)}
}

func Warning(ctx context.Context) *LogEntryBuilder {
    return &LogEntryBuilder{ctx: ctx, level: slog.LevelWarn, attrs: make([]slog.Attr, 0, 8)}
}

func Error(ctx context.Context) *LogEntryBuilder {
    return &LogEntryBuilder{ctx: ctx, level: slog.LevelError, attrs: make([]slog.Attr, 0, 8)}
}

func Fatal(ctx context.Context) *LogEntryBuilder {
    return &LogEntryBuilder{ctx: ctx, level: slog.LevelError + 1, attrs: make([]slog.Attr, 0, 8)}
}

type LogEntryBuilder struct {
    ctx   context.Context
    level slog.Level
    attrs []slog.Attr
    err   error
}

func (b *LogEntryBuilder) With(key string, value interface{}) *LogEntryBuilder {
    b.attrs = append(b.attrs, slog.Any(key, value))
    return b
}

func (b *LogEntryBuilder) WithError(err error) *LogEntryBuilder {
    b.err = err
    b.attrs = append(b.attrs, slog.Any("error", err))
    return b
}

func (b *LogEntryBuilder) Log(event string, msg string) {
    logger := slog.Default()
    
    // 添加 event 字段
    attrs := append([]slog.Attr{slog.String("event", event)}, b.attrs...)
    
    // 调用 slog
    logger.LogAttrs(b.ctx, b.level, msg, attrs...)
    
    // Fatal 行为
    if b.level >= slog.LevelError+1 {
        OsExit(1)
    }
}
```

#### 4.1.4 更新初始化逻辑

**修改**: `libs/xklib/klogging/nlogging.go`

```go
// 添加新的初始化函数
func SetDefaultSlogHandler(handler *SlogHandler) {
    logger := slog.New(handler)
    slog.SetDefault(logger)
}

// 保留旧的初始化函数（兼容）
func SetDefaultLogger(logger Logger) {
    defaultLogger = logger
}
```

#### 4.1.5 单元测试

**新文件**: `libs/xklib/klogging/slog_handler_test.go`

```go
package klogging

import (
    "bytes"
    "context"
    "encoding/json"
    "log/slog"
    "testing"
    "go.opentelemetry.io/otel/trace"
)

func TestSlogHandler_Basic(t *testing.T) {
    var buf bytes.Buffer
    handler := NewSlogHandler(&SlogHandlerOptions{
        Level:  slog.LevelInfo,
        Format: "json",
        Output: &buf,
    })
    
    logger := slog.New(handler)
    logger.Info("test message", slog.String("event", "TestEvent"))
    
    var logEntry map[string]interface{}
    json.Unmarshal(buf.Bytes(), &logEntry)
    
    if logEntry["event"] != "TestEvent" {
        t.Errorf("Expected event=TestEvent, got %v", logEntry["event"])
    }
}

func TestSlogHandler_Sampling(t *testing.T) {
    var buf bytes.Buffer
    handler := NewSlogHandler(&SlogHandlerOptions{
        Level:        slog.LevelInfo,
        SampledLevel: slog.LevelDebug,
        Format:       "json",
        Output:       &buf,
    })
    
    logger := slog.New(handler)
    ctx := context.Background()
    
    // 正常请求：Debug 不输出
    logger.DebugContext(ctx, "debug message")
    if buf.Len() > 0 {
        t.Error("Debug should not log without sampling")
    }
    
    // 采样请求：Debug 输出
    ctx = trace.ContextWithSpan(ctx, &mockSampledSpan{})
    logger.DebugContext(ctx, "debug message")
    if buf.Len() == 0 {
        t.Error("Debug should log when sampled")
    }
}

type mockSampledSpan struct {
    trace.Span
}

func (s *mockSampledSpan) SpanContext() trace.SpanContext {
    return trace.NewSpanContext(trace.SpanContextConfig{
        TraceID:    trace.TraceID{1, 2, 3},
        SpanID:     trace.SpanID{4, 5, 6},
        TraceFlags: trace.FlagsSampled,
    })
}
```

#### 4.1.6 Benchmark

**新文件**: `libs/xklib/klogging/slog_handler_bench_test.go`

```go
package klogging

import (
    "context"
    "io"
    "log/slog"
    "testing"
)

func BenchmarkSlogHandler(b *testing.B) {
    handler := NewSlogHandler(&SlogHandlerOptions{
        Level:  slog.LevelInfo,
        Format: "json",
        Output: io.Discard,
    })
    logger := slog.New(handler)
    ctx := context.Background()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        logger.InfoContext(ctx, "benchmark message",
            slog.String("event", "BenchEvent"),
            slog.Int("count", i),
        )
    }
}

func BenchmarkLogrusLogger(b *testing.B) {
    logrusLogger := NewLogrusLogger(context.Background())
    logrusLogger.SetConfig(context.Background(), "info", "json")
    SetDefaultLogger(logrusLogger)
    ctx := context.Background()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        Info(ctx).
            With("count", i).
            Log("BenchEvent", "benchmark message")
    }
}
```

**预期结果**:
```
BenchmarkSlogHandler-8      5000000    200 ns/op    0 allocs/op
BenchmarkLogrusLogger-8     1000000   1000 ns/op    3 allocs/op
```

---

### Phase 2: Gradual Migration (Week 3-5)

**目标**：逐步迁移现有代码，两种方式共存

#### 4.2.1 更新服务启动代码

**示例**: `services/smgapp/cmd/main.go`

```go
package main

import (
    "context"
    "log/slog"
    "os"
    
    "github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
)

func main() {
    ctx := context.Background()
    
    // 1. 初始化 OpenTelemetry
    klogging.InitOpenTelemetry()
    
    // 2. 创建 slog handler
    handler := klogging.NewSlogHandler(&klogging.SlogHandlerOptions{
        Level:        slog.LevelInfo,
        SampledLevel: slog.LevelDebug,
        Format:       os.Getenv("LOG_FORMAT"), // "json" or "text"
        Output:       os.Stderr,
        MetricsReporter: &metricsReporter{},
    })
    
    // 3. 设置为默认 logger
    klogging.SetDefaultSlogHandler(handler)
    
    // 4. 新代码可以直接用 slog
    slog.InfoContext(ctx, "Server starting",
        slog.String("event", "ServerStart"),
        slog.String("version", Version),
    )
    
    // 5. 旧代码继续用 klogging（兼容层）
    klogging.Info(ctx).
        With("version", Version).
        Log("ServerStart", "Server starting")
    
    // 两种方式输出完全一致！
}
```

#### 4.2.2 Middleware 更新

**新文件**: `internal/handler/middleware_tracing.go`

```go
package handler

import (
    "net/http"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/propagation"
    "go.opentelemetry.io/otel/trace"
    "github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
)

// OpenTelemetryMiddleware 提取 trace context（支持 W3C + B3）
func OpenTelemetryMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 1. 自动提取 trace context（支持多种格式）
        ctx := otel.GetTextMapPropagator().Extract(
            r.Context(),
            propagation.HeaderCarrier(r.Header),
        )
        
        // 2. 创建 span（可选，如果要完整分布式追踪）
        tracer := otel.Tracer("my-service")
        ctx, span := tracer.Start(ctx, "HTTP "+r.Method+" "+r.URL.Path)
        defer span.End()
        
        // 3. 兼容旧代码：同时填充 CtxInfo（过渡期）
        sc := span.SpanContext()
        if sc.IsValid() {
            ctxInfo := &klogging.CtxInfo{
                TraceID: sc.TraceID().String(),
                SpanID:  sc.SpanID().String(),
                Sampled: boolToSampledString(sc.IsSampled()),
            }
            ctx = klogging.NewContextWithCtxInfo(ctx, ctxInfo)
        }
        
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

func boolToSampledString(sampled bool) string {
    if sampled {
        return "1"
    }
    return "0"
}
```

#### 4.2.3 迁移指南文档

**新文件**: `libs/xklib/klogging/MIGRATION_GUIDE.md`

```markdown
# klogging 迁移指南

## 快速开始

### 旧代码（继续可用）
```go
klogging.Info(ctx).
    With("userId", userID).
    Log("UserLogin", "User logged in")
```

### 新代码（推荐）
```go
slog.InfoContext(ctx, "User logged in",
    slog.String("event", "UserLogin"),
    slog.String("userId", userID),
)
```

## 常见模式迁移

### 错误日志
**Before**:
```go
klogging.Error(ctx).
    WithError(err).
    With("orderId", orderId).
    Log("OrderFailed", "Order processing failed")
```

**After**:
```go
slog.ErrorContext(ctx, "Order processing failed",
    slog.String("event", "OrderFailed"),
    slog.String("orderId", orderId),
    slog.Any("error", err),
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
}
```
```

#### 4.2.4 集成测试

**新文件**: `libs/xklib/klogging/integration_test.go`

```go
package klogging_test

import (
    "context"
    "log/slog"
    "net/http"
    "net/http/httptest"
    "testing"
    
    "github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/propagation"
)

func TestOpenTelemetryPropagation(t *testing.T) {
    // 初始化
    klogging.InitOpenTelemetry()
    
    // 上游服务：注入 trace context
    req := httptest.NewRequest("GET", "/api/test", nil)
    ctx := context.Background()
    
    // 模拟 OpenTelemetry span
    // ... (创建 tracer provider, span)
    
    otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))
    
    // 下游服务：提取 trace context
    extractedCtx := otel.GetTextMapPropagator().Extract(
        context.Background(),
        propagation.HeaderCarrier(req.Header),
    )
    
    // 验证 trace ID 传播成功
    // ...
}
```

---

### Phase 3: Cleanup (Week 6)

**目标**：删除旧代码，完全迁移

#### 4.3.1 删除 logrus 依赖

```bash
# 1. 删除 logrus 相关文件
rm libs/xklib/klogging/logrus_logger.go
rm libs/xklib/klogging/logrus_logger_test.go
rm libs/xklib/klogging/simple_formatter.go

# 2. 更新 go.mod
go mod tidy  # 自动删除 github.com/sirupsen/logrus

# 3. 验证编译
go build ./...
```

#### 4.3.2 删除 CtxInfo（可选）

**条件**：所有代码已迁移到 OpenTelemetry

```bash
# 删除文件
rm libs/xklib/klogging/ctx_info.go
rm libs/xklib/klogging/ctx_info_test.go

# 删除兼容逻辑
# - SlogHandler.Enabled() 中的 CtxInfo 分支
# - SlogHandler.Handle() 中的 CtxInfo 分支
# - OpenTelemetryMiddleware 中的 CtxInfo 填充
```

#### 4.3.3 删除兼容层（可选）

**条件**：所有代码已迁移到 slog API

```bash
# 删除文件
rm libs/xklib/klogging/compat.go

# 更新文档
# - README.md: 移除旧 API 文档
# - 只保留 slog 使用说明
```

#### 4.3.4 性能验证

```bash
# 运行 benchmark
go test -bench=. -benchmem ./libs/xklib/klogging/

# 预期结果
# SlogHandler:     ~200 ns/op, 0 allocs/op
# LogrusLogger:    已删除
```

---

## 5. 代码示例

### 5.1 初始化（main.go）

```go
package main

import (
    "context"
    "log/slog"
    "os"
    
    "github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
    "github.com/xinkaiwang/shardmanager/libs/xklib/ksysmetrics"
)

func main() {
    ctx := context.Background()
    
    // 1. 初始化 OpenTelemetry
    klogging.InitOpenTelemetry()
    
    // 2. 初始化 logging
    handler := klogging.NewSlogHandler(&klogging.SlogHandlerOptions{
        Level:        slog.LevelInfo,
        SampledLevel: slog.LevelDebug,
        Format:       "json",
        Output:       os.Stderr,
        MetricsReporter: &MyMetricsReporter{},
    })
    klogging.SetDefaultSlogHandler(handler)
    
    // 3. 启动系统 metrics
    ksysmetrics.StartSysMetricsCollector(ctx, 15*time.Second, Version)
    
    // 4. 日志示例
    slog.InfoContext(ctx, "Server starting",
        slog.String("event", "ServerStart"),
        slog.String("version", Version),
    )
    
    // 5. 启动服务...
}
```

### 5.2 HTTP Middleware

```go
package handler

import (
    "net/http"
    "log/slog"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/propagation"
    "go.opentelemetry.io/otel/trace"
)

func TracingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 1. 提取 trace context
        ctx := otel.GetTextMapPropagator().Extract(
            r.Context(),
            propagation.HeaderCarrier(r.Header),
        )
        
        // 2. 创建 span
        tracer := otel.Tracer("my-service")
        ctx, span := tracer.Start(ctx, "HTTP "+r.Method+" "+r.URL.Path)
        defer span.End()
        
        // 3. 日志自动包含 trace ID
        slog.InfoContext(ctx, "Request received",
            slog.String("event", "HttpRequest"),
            slog.String("method", r.Method),
            slog.String("path", r.URL.Path),
        )
        
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### 5.3 业务代码

```go
package biz

import (
    "context"
    "log/slog"
)

func ProcessOrder(ctx context.Context, orderID string) error {
    slog.DebugContext(ctx, "Processing order",
        slog.String("event", "OrderProcessStart"),
        slog.String("orderId", orderID),
    )
    
    if err := validateOrder(orderID); err != nil {
        slog.ErrorContext(ctx, "Order validation failed",
            slog.String("event", "OrderValidationFailed"),
            slog.String("orderId", orderID),
            slog.Any("error", err),
        )
        return err
    }
    
    slog.InfoContext(ctx, "Order processed successfully",
        slog.String("event", "OrderProcessComplete"),
        slog.String("orderId", orderID),
    )
    return nil
}
```

---

## 6. 测试策略

### 6.1 单元测试

**范围**：
- `SlogHandler.Enabled()` — 动态级别判断
- `SlogHandler.Handle()` — Trace injection + Metrics
- `WithAttrs()` / `WithGroup()` — 链式操作

**工具**：
- `testing` 包
- `go.opentelemetry.io/otel/trace` mock

### 6.2 集成测试

**范围**：
- HTTP request → trace propagation → logging
- B3 headers → OpenTelemetry span → log output
- Metrics reporter 调用验证

**工具**：
- `httptest` 包
- OpenTelemetry test utils

### 6.3 性能测试

**范围**：
- Benchmark: SlogHandler vs LogrusLogger
- Allocation 检查（zero-allocation paths）
- 压测：1000 req/s 场景

**目标**：
- SlogHandler: <250ns/op
- Zero allocations for common paths

### 6.4 兼容性测试

**范围**：
- 旧 API (`klogging.Info()`) 输出正确
- CtxInfo → OpenTelemetry 转换正确
- B3 headers → trace ID 正确

---

## 7. 风险评估

### 7.1 技术风险

| 风险 | 影响 | 概率 | 缓解措施 |
|------|------|------|----------|
| slog 性能不达预期 | 中 | 低 | Phase 1 benchmark 验证 |
| OpenTelemetry 库 bug | 中 | 低 | 使用稳定版本（v1.24+） |
| 兼容层性能损耗 | 低 | 中 | 可删除（Phase 3） |
| Metrics 集成遗漏 | 高 | 中 | 集成测试全覆盖 |

### 7.2 业务风险

| 风险 | 影响 | 概率 | 缓解措施 |
|------|------|------|----------|
| 日志丢失 | 高 | 极低 | 渐进迁移，每阶段验证 |
| Trace 断链 | 中 | 低 | OpenTelemetry 自动传播 |
| 开发团队学习成本 | 低 | 中 | 提供 MIGRATION_GUIDE.md |

### 7.3 时间风险

| 风险 | 影响 | 概率 | 缓解措施 |
|------|------|------|----------|
| Phase 1 超期 | 低 | 低 | 核心功能简单，预留 buffer |
| Phase 2 超期 | 中 | 中 | 可分批迁移（按服务） |
| Phase 3 延迟 | 低 | 高 | 可选阶段，不影响功能 |

---

## 8. 回滚计划

### 8.1 Phase 1 回滚

**触发条件**：
- Benchmark 性能不达标
- 单元测试失败率 >5%

**回滚步骤**：
1. 删除新增文件（`slog_handler.go`, `otel.go`, `compat.go`）
2. 恢复 `go.mod`（移除 OpenTelemetry 依赖）
3. 保留现有 logrus 实现

**风险**：无（未影响生产）

### 8.2 Phase 2 回滚

**触发条件**：
- 生产环境日志丢失
- Trace 断链 >10%
- 性能下降 >20%

**回滚步骤**：
1. 服务重启，使用旧的 `LogrusLogger` 初始化
2. Middleware 切换回手动 CtxInfo 提取
3. 保留新代码（不删除），只改初始化逻辑

**风险**：低（两种方式共存）

### 8.3 Phase 3 回滚

**触发条件**：
- 发现兼容性问题
- 团队要求恢复旧 API

**回滚步骤**：
1. `git revert` Phase 3 提交
2. 恢复 `logrus_logger.go`, `ctx_info.go`, `compat.go`
3. `go mod tidy`（恢复 logrus 依赖）

**风险**：低（git 历史可追溯）

---

## 9. 里程碑

### Week 1-2: Phase 1 完成标志

- ✅ `SlogHandler` 实现通过所有单元测试
- ✅ Benchmark 结果 <250ns/op
- ✅ `InitOpenTelemetry()` 可用
- ✅ `compat.go` 兼容层可用

### Week 3-5: Phase 2 完成标志

- ✅ 至少 1 个生产服务迁移成功
- ✅ OpenTelemetry trace propagation 验证
- ✅ Metrics 数据正确（对比旧版本）
- ✅ MIGRATION_GUIDE.md 文档完成

### Week 6: Phase 3 完成标志（可选）

- ✅ 所有服务迁移完成
- ✅ logrus 依赖删除
- ✅ CtxInfo 删除（可选）
- ✅ 性能测试通过

---

## 10. 附录

### 10.1 依赖版本

```go
// go.mod
require (
    log/slog                                      // Go 1.21+ 标准库
    go.opentelemetry.io/otel v1.24.0             // OpenTelemetry 核心
    go.opentelemetry.io/otel/trace v1.24.0       // Trace API
    go.opentelemetry.io/contrib/propagators/b3 v1.24.0  // B3 支持
)

// 可选（如果要完整分布式追踪）
require (
    go.opentelemetry.io/otel/sdk v1.24.0
    go.opentelemetry.io/otel/exporters/jaeger v1.24.0
)
```

### 10.2 参考资料

- **log/slog 官方文档**: https://pkg.go.dev/log/slog
- **log/slog 设计文档**: https://go.dev/blog/slog
- **OpenTelemetry Go**: https://opentelemetry.io/docs/instrumentation/go/
- **B3 Propagation**: https://github.com/openzipkin/b3-propagation
- **W3C Trace Context**: https://www.w3.org/TR/trace-context/

### 10.3 联系人

- **负责人**: 王总
- **实施者**: 大白 (Bruce)
- **审核者**: TBD

---

**最后更新**: 2026-03-01  
**下次审核**: Phase 1 完成后
