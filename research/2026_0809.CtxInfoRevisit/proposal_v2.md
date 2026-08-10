# klogging 补全提案 v2（Fix-Forward）

**日期**: 2026-08-09
**前置阅读**: [notes.md](notes.md)（问题编号 KLOG-001 ~ 012 的定义在那里）
**性质**: 这不是重做迁移，而是补上 `9484668` 迁移留下的缺口。

---

## 0. Revert vs Fix-Forward 决策

| | Revert 到 `9484668^` | Fix-Forward（本提案） |
|---|---|---|
| 解决 12 个 KLOG 问题 | ❌ 回到旧问题集（CtxInfo 并发 workaround、logrus、3 allocs/log） | ✅ 逐个解决 |
| 改动范围 | 所有服务的全部日志调用点回滚 + 与 c5e17c2/ca3a8c6 冲突 | 集中在 klogging + krunloop + 各 main.go |
| 未来 | 迟早还要再迁移一次 | 一次到位 |

**结论：Fix-Forward。**

**v1 计划的教训**（避免重犯）：
1. 1100 行计划里贴了大量代码清单，实现时全部走样 → 本提案只写决策和验收标准，不贴实现代码。
2. "OpenTelemetry handles trace IDs automatically" 这类假设没验证就写进注释 → 本提案每个 Phase 有可验证的验收标准。
3. 计划里"必须保留"的功能（metrics）实际丢了没人发现 → 本提案每个丢失能力对应一个 KLOG 编号，做完销号。

---

## 1. 核心洞察（提案的三个技术支点）

### 支点 A：装 OTel SDK ≠ 必须上导出/后端
`TracerProvider` 可以**不配 exporter** 运行：span 不上报任何后端，但本地会生成
真实的 trace_id/span_id + 执行采样决策。这正好满足"日志有 trace 关联"的最小需求，
成本几乎为零。等将来要看 Jaeger/Tempo 时再加一行 exporter 配置。
→ 这使 KLOG-005 从"重决策"降级为"轻决策"：先装 SDK（no-export 模式），后端另议。

### 支点 B：KLOG-002 不需要把过滤挪进 Handle
slog.Logger 对**每条**日志（包括被压掉的）都会调 `Enabled(ctx, level)`——
被压掉的日志到不了 `Handle`，但一定经过 `Enabled`。所以：
- `Enabled` 里统计"尝试数"（按 level 粒度，此时拿不到 event/attrs）；
- `Handle` 里统计"输出数"（按 level+event 全粒度）；
- 压掉的量 = 尝试数 − 输出数。
零性能代价（不用为被压日志组装 attrs），代价仅是压掉量只有 level 粒度——可接受。

### 支点 C：runloop 每事件独立 root span【2026-08-09 简化定稿】
runloop 定位是 daemon：每个事件在**自己的** trace/span 里运行，不追溯投递方
（王总裁定：来源请求 id 不重要）。
- runloop `Process` 前 `tracer.Start(ctx, eventName)`，`defer span.End()`——改动收缩在
  runloop.go 一处，**`IEvent` 接口不动**；
- 事件处理期间日志自动带该事件的 trace_id；
- 事件作为 root span 走 sampler → 按采样率自动有事件留下 Debug 现场，
  与 HTTP 请求机制完全一致；
- 副产品：事件处理耗时天然成为 span，与 `RunLoopElapsedMsMetric` 互补。
- （曾考虑 span link 回投递方：不做；将来若需要是纯增量，不破坏现有设计。）

---

## 2. 分阶段计划

### Phase 0 · 现状盘点（KLOG-012）— 0.5 天
- 列出所有 ctx 入口：HTTP handler / etcd watch / 定时器 / runloop 事件。
- 每类入口回答：ctx 里有无 trace？日志 trace_id 覆盖率多少？
- 顺手确认各服务 middleware 是否做 `Extract`。
- **产出**：notes.md 追加盘点表；KLOG-005/011 的方案参数由此确定。

### Phase 1 · Trace 完整性（KLOG-005 + 011）— 2~3 天 ★核心
1. klogging 增加 `InitTracerProvider(serviceName, samplerRatio)`：
   SDK TracerProvider + `ParentBased(TraceIDRatioBased)` sampler，**默认无 exporter**（支点 A）。
   显式传 WithSampler/WithSpanLimits/WithResource 封死 env 隐藏配置通道（KLOG-005b ③-3）；
   附带自定义 IDGenerator（~30 行，形状同官方 mutex+PRNG，仅改播种失败为 fail-fast，
   修 KLOG-005b ③-1；锁保留——王总裁定 PRNG 临界区纳秒级可接受）。
2. 各服务 main.go 接上；HTTP middleware 统一为 Extract + Start span。
   **KLOG-014（修正版）**：propagator = `TraceContext{}` + B3（王总明确需要 B3，
   保留）；仅移除 `propagation.Baggage{}`——被否决的跨服务字段通道不得处于
   注册状态。将来需要时加回 = 显式 diff。
   **命名（2026-08-09 定案）**：`InitOpenTelemetry` → `InitDefaultPropagator`，
   新函数命名 `InitDefaultTracerProvider`——窄名各认领一环（格式 / 引擎），
   杜绝"调了一个函数就以为整个 OTel 就绪"的误导（KLOG-011 教训）；
   8 个调用点同批机械替换。
3. runloop：每事件开独立 root span（支点 C 简化版）——只改 runloop.go 事件循环，
   `IEvent` 接口不动。
4. 顺手修 KLOG-006（sampled 取 `min(globalLevel, sampledLevel)`）。
- **验收**：
  - 任一 HTTP 请求（带或不带上游 trace header）→ 全链路日志有一致 trace_id；
  - 任一 runloop 事件处理期间的日志有一致 trace_id，且 span link 指向投递请求；
  - 本地采样生效：sampled 请求打出 Debug 日志（无需上游标记）。

### Phase 2 · 可观测性回填（KLOG-001 + 002 + 010）— 1~2 天
1. 决定 metrics 去留（**建议：留**，日志量指标是容量管理刚需）。
2. 按支点 B 重构，指标形状（2026-08-09 定稿）：**单指标 + drop tag**：
   - `log_count{level, event, drop}`，`drop=0/1`；
   - `Handle` 里记 `drop=0`（全字段）；`Enabled` 返回 false 时记 `drop=1`
     （此处拿不到 event/attrs，event 填空串，size 记 0 —— 这是结构性约束，如实呈现）；
   - 接口保持单方法：`ReportLog(ctx, level, event string, size int, dropped bool)`。
   - 查询：总尝试量 = sum(log_count)，压掉量 = sum(log_count{drop="1"})。
3. 删除孤儿 `MyLoggerMetrcsReporter`，在 shardmgr 用 kmetrics 实现新接口并接线。
4. KLOG-010：`globalLevel` 换 `slog.LevelVar`，暴露 `SetLevel(string)`
   （运行时改级别；是否加 HTTP admin 端点由服务侧自定）。
- **验收**：metrics 端点能看到 log_count{level,event}；压掉量可由两指标相减得出；
  运行时 SetLevel 立即生效。

### Phase 3 · Ambient 字段回归（KLOG-007）— 1.5 天 【设计定稿 2026-08-09，三层结构】

设计原则：旧 CtxInfo 的病根是把"写一次读多次的分层字段"和"多模块并发读写的共享对象"
塞进同一个带锁结构。拆成三层，各自用对的并发策略：

1. **immutable attrs 链 + Importance 分级**（分层日志字段）：
   - `CtxWithAttrs(ctx, attrs...)` / `CtxWithAttrsLevel(ctx, importance, attrs...)`；
   - 新节点指向父节点，`context.WithValue` 存；Handler 遍历链注入（子覆盖父）；
   - Importance 三级保留（High=全带 / Mid=Debug 带 / Low=Verbose 带）——
     xklib 是库，其他项目依赖此能力，不以本仓库用量判断取舍；
   - immutable → 零锁。
2. **`AttrProvider` 接口**（请求级可变对象，costCenter 类场景）：
   - `type AttrProvider interface { LogAttrs(level slog.Level) []slog.Attr }`；
   - `CtxWithProvider(ctx, p)` 把对象指针挂进链；各模块随时取出修改
     （对象自管并发：atomic/mutex 按访问模式选）；
   - Handler 打日志**时刻**调 `LogAttrs()` → 日志带最新累计快照；
   - 比旧 ModifyByKey 强：任意结构化对象 + log 时求值 + 并发策略局部化。
3. **`ModifyByKey`：❌ 移除**（王总授权：并发风险大可删）。
   场景归宿：回填/累加 → 第 2 层 provider；下游改写标量 → 第 1 层同 key 覆盖。

- **路线 b（OTel Baggage）：❌ WONTFIX（2026-08-09 王总否决）** —— Handler 不读 baggage。
- **验收**：
  - biz 层 `CtxWithAttrs` 后，dao 层 `slog.InfoContext` 日志自动带该字段；
  - Mid 级字段仅在 Debug 输出时出现；
  - costCenter demo：多模块累加后，任意时点日志带最新累计值，`go test -race` 通过。
- 配套约束：依赖调用点用 `XxxContext` 变体传 ctx —— S14（trace-blind-log）扫描持续盯住。

### Phase 4 · 清理（KLOG-003 + 004 + 008 + 009）— 0.5 天
- KLOG-003【✅ D3 已决策 2026-08-09：要】：加 `klogging.Fatal(ctx, msg, attrs...)`
  （log + OsExit(1)，生产真退出，Mock 仅测试）。实现注意：os.Exit 不跑 defer，
  依赖 Handler 同步写 stderr 落盘——写注释钉死此前提 + 测试断言
  （MockOsProvider 触发前日志已可读），防将来 Output 换缓冲/异步 writer 丢临终日志。
- KLOG-004：README 去掉 CtxInfo 段落；REFACTOR_PLAN.md 头部标注
  "已完成，后续见 research/2026_0809.CtxInfoRevisit/"。
- KLOG-008：HandlerOptions 加文档注释说明零值约定（不改类型，代价最小）。
- KLOG-009：`StopAndWaitForExit(ctx)` 接受调用方 ctx；内部日志改用之。

---

## 3. 顺序与依赖

```
Phase 0 ──> Phase 1 ──> Phase 2 ──> Phase 4
   (盘点)     (trace)  (metrics)    (清理)
                └────────────> Phase 3 (baggage, 可延后)
```

每个 Phase 独立可合并、独立可回滚。总量 5~7 天，其中 Phase 1 是价值大头。

## 4. 待王总拍板的决策点

| # | 决策 | 建议 | 影响 |
|---|------|------|------|
| D1 | ~~装 OTel SDK（no-export 模式）？~~ | ✅ 已决策（2026-08-09）：装 | 经 demo + KLOG-005a/005b 验货后定案 |
| D2 | 日志 metrics 留不留？ | 留 | 不留则 Phase 2 变成纯删除，KLOG-002 销号为 WONTFIX |
| D3 | ~~Fatal 语义要不要？~~ | ✅ 已决策（2026-08-09）：要，prod 真退出，Mock 仅测试 | 同步落盘前提写注释+测试钉死 |
| D4 | ~~Phase 3 现在做还是等需求？~~ | ✅ 已决策（2026-08-09）：做，路线 a 先行 | 王总确认 biz→dao 场景为真实需求 |
