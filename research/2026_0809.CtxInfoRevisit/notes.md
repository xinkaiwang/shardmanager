# CtxInfo Revisit — klogging 重构收尾讨论

**日期**: 2026-08-09
**参与**: 王总 + Claude
**背景**: klogging 已完成 logrus+CtxInfo → slog+OpenTelemetry 迁移（commit `9484668`），
本文记录迁移后的遗留问题清单。每个问题有唯一编号 KLOG-NNN，后续讨论/commit 引用编号。

---

## 1. CtxInfo 原理回顾（已删除的旧机制）

旧 `ctx_info.go` 是挂在 context 上的**链式 KV 包**：

```
ctx (root)
 └─ CtxInfo{Details: {trace_id, sampled...}, Parent: nil}
     └─ 子 ctx → CtxInfo{Details: {workerId...}, Parent: ↑}   // 继承靠指针，不复制
```

- 非导出 key 的 `context.WithValue` 存 `*CtxInfo`；节点含 `Parent` 指针 + `Details map[string]*KVL`。
- 每条 KV 带重要性等级：`HighImportance`（所有日志都带）/ `Mid`（Debug 才带）/ `Low`（Verbose 才带）。
- 打日志时 `NewEntry()` → `VisitForward()` 从最老祖先向下遍历整条链，把等级达标的 KV 附加到日志行（子覆盖父）。
- `FindByKey` 沿链向上查；`ModifyByKey` 用 mutex + 整体覆盖 map 防并发。

**CtxInfo 承担了三个角色**：

| 角色 | 迁移后的接替者 | 状态 |
|------|--------------|------|
| R1: Ambient 日志字段（一次 With，下游日志全带上） | ❌ 无接替 | 能力丢失 → KLOG-007 |
| R2: Trace 传播（B3 header → trace_id/sampled） | ✅ OTel propagator + Handler 注入 | 部分接替 → KLOG-005/011 |
| R3: 采样提级（sampled=1 → 打 Debug 日志） | ✅ Handler.Enabled 查 span.IsSampled() | 已接替（有 nit → KLOG-006） |

---

## 2. 现状

- 三个 Phase 实际已执行完，且比计划激进：**没走兼容层**，所有服务直接用
  `slog.XxxContext` + `klogging.NewHandler`（旧链式调用只剩注释）。
- logrus / ctx_info.go / compat 层均不存在。
- 现存文件：`handler.go` `level.go` `os.go` `otel.go` + tests。

---

## 3. 问题登记表（Issue Registry）

> 状态: OPEN / DISCUSSING / DECIDED / DONE / WONTFIX

### KLOG-001 · Metrics 集成断链 【P1 · OPEN】
- `Handler.MetricsReporter` 接口存在（[handler.go:24](../../libs/xklib/klogging/handler.go#L24)），
  但**没有任何服务传 `Metrics:`**（smgapp/helloblitz/hello 的 main.go 都只传 Level/Format）。
- `services/shardmgr/service/shardmgr/metrics_reporter.go` 的 `MyLoggerMetrcsReporter`
  实现的是**旧接口**（`ReportLogSizeBytes`/`ReportLogErrorCount`），与新接口
  `ReportLog(ctx, level, event, size, logged)` 对不上，现为孤儿代码。
- 这正是 REFACTOR_PLAN 风险表里"Metrics 集成遗漏（影响：高）"的实际发生。
- 选项：(a) 改成新接口并在各 main.go 接上；(b) 两边一起删。

### KLOG-002 · `logged` 标志永远为 true 【P1 · OPEN · 结构性】
- [handler.go:119](../../libs/xklib/klogging/handler.go#L119) `logged := h.Enabled(ctx, r.Level)`。
- slog.Logger 在调 `Handle` **之前**就先调 `Enabled`，被过滤的日志根本到不了 `Handle`
  → `logged` 恒真，"被压掉的日志量"这个指标结构上不可能统计到。
- 旧系统能统计（旧 NewEntry 无论是否输出都会走 metrics 路径）。
- 修法：把过滤从 `Enabled` 挪进 `Handle`（`Enabled` 恒真 → `Handle` 里先计数、再决定是否
  委托 baseHandler）。代价：所有日志（含被过滤的）都要走 attr 组装，有性能开销。
- 与 KLOG-001 绑定：先决定 metrics 还要不要，再决定此项修不修。

### KLOG-003 · Fatal 语义悬空 【P2 · OPEN】
- [level.go:16](../../libs/xklib/klogging/level.go#L16) 定义了 `LevelFatal`；
  [os.go](../../libs/xklib/klogging/os.go) 保留了可 mock 的 `OsExit`——但没有任何代码把两者接起来。
- 旧 API `Fatal(ctx).Log()` 会 log + 退出进程；现在 `OsExit` 零调用方。
- 选项：(a) 加 `klogging.Fatal(ctx, msg, attrs...)` 辅助函数（log + OsExit(1)）；
  (b) 删掉 LevelFatal + os.go。

### KLOG-004 · 文档过期 【P3 · OPEN】
- [README.md:162](../../libs/xklib/klogging/README.md#L162) 还在教已删除的 `GetCtxInfoFromCtx`。
- REFACTOR_PLAN.md 应标记"已完成"并归档（它已是历史文档，且和实际实现有出入：
  实际没做 compat 层、接口签名也变了）。

### KLOG-005 · 只有 propagator，没有 TracerProvider/SDK 【P1-P2 · DISCUSSING · 设计决策】
- `InitOpenTelemetry()`（[otel.go](../../libs/xklib/klogging/otel.go)）只配了 propagator。
- 效果：上游带 trace header 的请求，Extract 后 ctx 里是 remote SpanContext，
  日志 trace_id 注入 ✅；但服务**自己创建 span、本地采样决策、上报 span** 都是 no-op。
- 也就是说"sampled 请求打 Debug 日志"只在**上游已标记 sampled** 时生效，本服务永远
  不会主动发起采样。
- 待决策：目标是 (a) 仅"日志带上游 trace_id"（现状够用）还是 (b) 完整分布式追踪
  （需装 SDK：TracerProvider + Sampler + Exporter）。
- 附带审计项：确认每个服务的 HTTP middleware 确实在做 `Extract`（未逐一验证）。

### KLOG-005a · OTel SDK 热路径审计（D1 验货记录，2026-08-09）
- 背景：王总对开源项目热路径质量的合理不信任（例：顶层 `rand.Float64()` 在 go1.21
  是全局锁——Claude 的选项 B 草图恰好犯了这个错，证明此类错误写起来毫无阻力）。
- 逐环节验货（otel-go v1.24，每次 `tracer.Start`）：
  - 采样决策 `TraceIDRatioBased`：对 trace_id 前 8 字节整数比较，纯函数无锁无 rand ✅
    （确定性推导附赠分布式一致采样；与 kmetrics 无锁读同哲学）；
  - ID 生成：⚠️ 默认 IDGenerator 为 mutex + `*rand.Rand`（per-TracerProvider 全局锁，
    临界区约取 24 字节随机数）。量级上距瓶颈差数个数量级；逃生舱
    `sdktrace.WithIDGenerator(...)` 可无 fork 替换；
  - span 对象：命中一次堆分配，未命中轻量 nonRecording；
  - 后台活动：no-export 模式下为零（无 batch processor / 导出队列 / 后台 goroutine）。
- 结论：依赖面小到可逐行审计，每块有替换舱口——比"信任"更可靠。
- 附带 audit 项：`kcommon.RandomString` 疑似用全局 math/rand（go1.21 带锁），
  目前不在热路径，入清单备查。

### KLOG-005b · otel trace SDK 全量审计（2026-08-09，核心 ~3100 行逐块过）
- **①有我们无**：span 模型全套 / 采样框架 / SpanLimits 有界保护（evictedQueue）/
  runtime-trace 集成（go tool trace 可见业务 span）/ env 配置通道。
- **②我们有它无**：ambient 分层 KV+Importance、log 时刻求值的 AttrProvider
  （costCenter）、sampled→日志提级、可读前缀 id、kcommon TimeProvider 集成
  （SDK 直接 time.Now，span 时长不可 mock——我们不导出 span，影响≈0）。
  结论：无丢失项，缺口均已在 Phase 3 计划内或不适用。
- **③问题（均带 failure scenario）**：
  - ③-1 [id_generator.go:74] `_ = binary.Read(crand.Reader,...)` 错误静默忽略 →
    失败时 seed=0 固定序列 → 多实例同败时产出相同 ID 序列，跨 pod 日志静默
    join 成同一"trace"。kcommon 同场景响亮崩，otel 无声错——最差失败模式。概率极低。
  - ③-2 [id_generator.go:52,62]+[tracer.go:86-96] per-provider mutex 包 randSource，
    且 ID 生成先于采样判断——每次 Start 无条件过锁（采样率救不了）。临界区 8~24 字节
    PRNG 读，比 opencensus channel 病轻三个数量级。
  - **③-1/③-2 同解**：`WithIDGenerator` 自换实现（~30 行），可进 Phase 1 或记账。
  - ③-3 env 通道（OTEL_TRACES_SAMPLER 等）已验证：env 先应用、代码 opts 后应用，
    显式配置胜出 [provider.go:110-114]。缓解：InitTracerProvider 显式传全字段封死。
  - ③-4 [batch_span_processor.go:400-404] 队列满静默丢，仅 atomic 计数 + Debug 日志，
    无 metric（S2-F 三角不完整）。no-export 不涉；将来接 exporter 必须自补 drop metric。
  - ③-5 观察：Tracer() 过 mutex+map（包级缓存绕开）；每 Start 1~2 次小分配；
    span 用 per-span 锁（设计正确）。
- **总评**：无 opencensus 病（无 channel/单 worker/每记录大分配）；一个真缺陷（③-1）
  与唯一的锁（③-2）共享同一个 30 行解法。

### KLOG-006 · sampled 提级应取 min 【P3 · OPEN · nit】
- [handler.go:92-101](../../libs/xklib/klogging/handler.go#L92-L101)：sampled 分支直接
  `return level >= h.sampledLevel`。
- 若全局级别设得比 sampledLevel 还低（如 verbose < debug），被采样的请求反而打得**更少**。
- 修法：sampled 时用 `min(globalLevel, sampledLevel)`。

### KLOG-007 · CtxInfo 角色 R1（ambient ctx 字段）能力丢失 【P2 · DECIDED→做】
- 旧能力：`info.With("workerId", x)` 一次，此 ctx 链下游所有日志自动带 `workerId`。
- slog 的 `logger.With()` 挂在 **logger** 上而非 ctx 上，穿函数边界需要传 logger，不等价。
- 目前无代码依赖（原唯一用户 solver_group.go 已迁移），但业务代码复杂后大概率会想要。
- 候选方案：
  (a) **OTel Baggage**（标准答案）：propagator 已含 `propagation.Baggage{}`，
      在 Handler.Handle 里读 `baggage.FromContext(ctx)` 注入日志字段。跨服务还能自动传播。
  (b) 自建轻量 ctx-attrs（context.WithValue 存 []slog.Attr + Handler 读取）。
  (c) 不做，需要时用 `logger.With` 显式传 logger。
- 倾向 (a)：与 OTel 生态一致，且是唯一能跨服务传播的方案。

### KLOG-008 · HandlerOptions 零值歧义 【P3 · OPEN】
- [handler.go:44-49](../../libs/xklib/klogging/handler.go#L44-L49)：`opts.Level == 0` 判默认，
  但 `slog.LevelInfo == 0` → "未设置"与"显式 Info"不可区分。
- 实际后果：`SampledLevel` 无法显式设为 Info（会被覆盖成 Debug）。Level 侧恰好无害
  （默认就是 Info）。
- 修法：Options 字段改 `*slog.Level`，或加 `LevelSet bool`，或文档写明约定。

### KLOG-009 · xklib 内部日志用 context.Background() 【P3 · OPEN · 关联 S14】
- [unbounded_queue.go:121](../../libs/xklib/krunloop/unbounded_queue.go#L121)、
  [runloop.go:164](../../libs/xklib/krunloop/runloop.go#L164) 用
  `slog.XxxContext(context.Background(), ...)` → 无 trace 关联（trace-blind-log）。
- 属生命周期日志，危害有限，但 `StopAndWaitForExit` 可以接受调用方 ctx。

### KLOG-010 · 运行时改日志级别的能力丢失 【P2 · OPEN】
- 旧 `LogrusLogger.SetConfig(ctx, level, format)` 支持运行时改级别/格式。
- 新 Handler 的 `globalLevel`/`sampledLevel` 是构造后不可变的普通字段。
- 修法：用 `slog.LevelVar`（标准做法）替换 `globalLevel`，暴露 `SetLevel(string)`；
  运维场景（线上临时开 debug）很常用。

### KLOG-011 · runloop 事件丢失了 trace 关联 【P1 · DISCUSSING · 疑似最大暗坑】
- 旧行为：runloop 处理每个事件前 `klogging.EmbedTraceId(ctx, "rl_"+epochId)` ——
  每次事件处理有唯一合成 trace id，**该事件处理期间的所有日志可以用 rl_N join 起来**。
- 新行为：[runloop.go:130](../../libs/xklib/krunloop/runloop.go#L130) 注释
  "EmbedTraceId removed - OpenTelemetry handles trace IDs automatically" ——
  但这个说法**不成立**：runloop 事件是后台异步处理，ctx 里没有 remote span，
  而本服务又没有 TracerProvider（KLOG-005），所以 OTel 什么都不会注入。
- 实际后果：**所有经 runloop 的后台处理日志完全没有 trace 关联**，退化程度超过旧系统。
  这类日志恰恰是 shardmgr 核心路径（事件驱动架构，几乎所有状态变更走 runloop）。
- 修法（依赖 KLOG-005 的决策）：
  (a) 装 OTel SDK 后，runloop 在 `event.Process` 前为每个事件开 span
      （`tracer.Start(ctx, eventName)`），日志自动带 trace_id，且事件处理耗时天然成为 span；
  (b) 不装 SDK 的轻量版：恢复合成 id（存 ctx，Handler 里作为 fallback 注入）。
- 另一个丢失的关联：event 若由某个带 trace 的请求投递（PostEvent），旧系统靠
  CtxInfo 链**跨越队列边界**保留来源 trace；现在 PostEvent 不携带 ctx，
  请求→异步处理的因果链完全断开。若要恢复，需要 event 创建时捕获
  `trace.SpanContext` 并在 Process 时 `trace.ContextWithSpanContext` 接回（即 span link 模式）。

### KLOG-012 · 审计：各服务 trace 入口现状盘点 【P2 · 核心已完成 2026-08-09】
- **盘点结果（git 全仓验证）**：
  - 当前代码：`Extract` / `tracer.Start` / `SpanFromContext` 在 services 和 xklib 中
    **零调用** → Handler 的 trace 注入代码从未触发过，**当前日志 trace_id 覆盖率 = 0%**。
  - 旧系统（`9484668^`）实际拥有的也远比记忆中少：
    - trace_id 生成仅两处：hello.go demo（RandomString(8)）、runloop（"rl_"+epochId）；
      **services 的 HTTP 入口从未生成/提取过 trace_id**；
    - span_id 从未存在（CtxInfo 只有 traceId 一个字符串，无 span 概念）；
    - B3 header 提取整个仓库零命中（v1 计划里的"B3 采样"是愿景不是现状）；
    - sampled 标志无任何代码设置过。
  - 结论：旧系统 = 自制关联 ID 管道（消费端）+ 两个手工生成点；重构升级了消费端
    （Handler 读 OTel SpanContext，方向正确），删除了仅有的生成点且未装替代（SDK）。
- **对 D1 的重构性影响**：D1 不是"引入大系统"，而是"生成端选标准件（OTel SDK
  no-export）还是重写土法（RandomString + ctx fallback）"——工作量相近
  （都是初始化/runloop/middleware 三触点），差别在 ID 是否为标准格式、
  能否复用现成消费端、能否本地采样。
- 剩余盘点项（低优先）：etcd watch / 定时器入口的 ctx 来源梳理，Phase 1 实施时顺手做。

### KLOG-013 · kcommon.RandomXxx 审计 【P3 · 改判 2026-08-09 · 包：kcommon】
（初判"三连问题"，经王总两轮质疑后改判——ID 需要唯一性而非不可预测性；
真熵采集速率极低（headless 机器每秒个位~百级 bits），"每 ID 耗真熵"从根上不成立；
内核 5.6+ 的 getrandom 本身就是 ChaCha20 DRBG = "crypto 种子 + PRNG"同架构。
教训：别在数据路径上碰真熵——老内核 /dev/random 阻塞导致的启动卡死是一代生产事故。）
- **① 撤回**："身份类默认应 crypto/rand"不成立。kcommon 的 crypto-seed + math/rand
  对 ID/jitter 用途是正确设计（otel 默认 IDGenerator、内核 DRBG 同架构）。
  待办仅剩：注释写明契约「非 secret 级，不得用于 token/凭证」，防未来误用。
- **② 降级为卫生问题**：[rand_util.go:42] 种子进 INFO 日志——在无 secret 用途契约下
  无实际危害；该行对 operator 也无价值（confirmation-only）。删掉或降 Debug，不紧急。
  （确定性重放对测试反而可能是特性。）
- **③ 维持（唯一真缺陷）**：crypto_rand.Read 失败 → 仅 Warn → `op(nil)` → 回调内
  空指针 panic。代码写的是"降级"，跑起来是"崩溃"。触发概率极低（现代内核
  getrandom 几乎不失败），顺手修。
- 结论：正字法三分：secret → crypto/rand；ID → 好种子+PRNG；jitter → PRNG。
  对 D1 无影响：otel 默认实现正是被认可的设计。
- **根因复盘（方法论）**：初判失误 = 标签化判断（"crypto=好/伪随机=坏"）替代了
  后果链推导——没有先问"消费者需要什么性质"，威胁模型与数量级计算双缺席，
  但置信语气满格。纠偏依赖了王总两轮质疑才发生 → 这正是"高置信标签化"
  在开源生态中危险的原因（缺乏对等质疑时错误结论直接合入）。
- **登记表新规则（自此生效）**：没有具体 failure scenario（谁/什么场景/什么损害），
  不得赋 severity；写不出后果链的只能记"观察"，不得记"问题"。

### KLOG-014 · propagator 面收窄：移除 Baggage 与 B3 注册 【P2 · DECIDED 2026-08-09】
- 发现：`propagation.Baggage{}` 已注册在 otel.go——被否决的"字段自动跨服务"功能
  处于半武装状态：middleware Extract 上线后，外部 `baggage` header 即被解析进 ctx；
  任何同事三行 `baggage.ContextWithBaggage(...)` 即激活出站注入。危险不在业务代码
  三行里，在 propagator 注册里——review 拦不住（"dangerously simple to get enabled"）。
- ~~B3 同理：本仓库零使用，疑似 cargo~~ **【修正 2026-08-09】B3 保留——王总明确需要**。
  初判依据"本仓库 grep 零命中"是错误方法：xklib 是库，使用者在仓库之外
  （与 Importance 分级同款错误，同日第二次犯）。
- **决策（修正版）**：`InitOpenTelemetry` = `TraceContext{}` + B3，仅移除
  `propagation.Baggage{}`。将来需要 Baggage 时加回 = 显式 diff + review。
- 原则：特性面 = 攻击面，未使用的能力不得处于注册状态（配方携带未定价功能
  = log4j JNDI 的进入路径）。
- Failure scenario（若不修）：外部客户端以 `baggage: k=v...`（≤180 条×4KB）灌入
  ctx（每请求解析成本+内存）；未来任何 baggage→log 桥被无害提交激活后，
  攻击者可伪造日志字段（如 runloop=core）污染按字段查询的信任基础。

---

## 4. 依赖关系 & 建议处理顺序

```
KLOG-005 (要不要 OTel SDK?) ──决定──> KLOG-011 (runloop trace 方案)
        │                              KLOG-007 (Baggage 方案可行性)
        └──输入来自── KLOG-012 (现状盘点)

KLOG-001 (metrics 要不要?) ──决定──> KLOG-002 (Enabled/Handle 结构)

独立可做：KLOG-003, 004, 006, 008, 009, 010
```

建议：先做 KLOG-012 盘点（半天），带着数据决策 KLOG-005，其余顺势展开。

---

## 5. 决策记录（Decision Log）

| 日期 | 编号 | 决策 | 理由 |
|------|------|------|------|
| 2026-08-09 | KLOG-002 | metrics 语义改为显式双指标：`log_emitted_count{level,event}` + `log_dropped_count{level}`，接口拆 `ReportEmitted`/`ReportDropped` | 王总提出 "dropped" 更可读；构成完整 in/out/drop 三角（attempted = emitted + dropped）。注：`Enabled` 方法名是 slog.Handler 接口契约，不可改 |
| 2026-08-09 | KLOG-007 | D4 改判：**现在做**，路线 a（自建 immutable ctx-attrs 链）先行，Baggage 等跨服务需求 | 王总确认 biz→dao 注入 ctx 字段是真实需求；该场景为进程内，路线 a 足够且无旧 CtxInfo 的并发问题 |
| 2026-08-09 | — | klogging 定位确认：slog 的 provider 层（Handler + 配置辅助），业务只面对标准 slog API | 副产品：第三方库的 slog 日志也自动获得 trace 注入 + metrics |
| 2026-08-09 | **D1** | **✅ 定案：选 A——装 OTel SDK（no-export 模式）**，Phase 1 附带自定义 IDGenerator | 经 demo 实证 + KLOG-005a/005b 两轮验货。自定义 IDGenerator 的理由仅剩 ③-1（种子失败须响亮：进程启动时 fail-fast，同 kmetrics tag 冲突 os.Exit 哲学）；锁不是理由——王总：PRNG 临界区纳秒级，全局锁可接受（opencensus 病根是锁后挂 channel+单消费者，不是锁本身） |
| 2026-08-09 | KLOG-014 | **✅ 命名定案**：`InitOpenTelemetry` → `InitDefaultPropagator`（只管 header 格式）；Phase 1 新增 `InitDefaultTracerProvider`（只管 span 引擎/采样）。与 Baggage 摘除同批在 Phase 1 执行（8 个调用点机械替换） | "Default" 前缀 = 库预组装默认件、可自行组装原件（同 slog.SetDefault / SetAsDefault 方言）；旧名 "OpenTelemetry" 过度承诺，暗示"OTel 已就绪"——疑似 KLOG-011 错误注释（"OTel handles trace IDs automatically"）的帮凶。窄名各认领一环，名实相符 |
| 2026-08-09 | **D3** | **✅ 定案：要 Fatal**——`klogging.Fatal(ctx, msg, attrs...)` = log + `OsExit(1)`；生产真退出，Mock 仅测试 | 王总：fatal 在 prod 就该 os.Exit(1)。实现钉死一个前提：os.Exit 不跑 defer，Fatal 日志依赖 Handler 同步写 stderr 才能落盘——此前提写注释 + 测试断言，防将来有人给 Output 套缓冲/异步 writer 丢掉临终遗言 |
| 2026-08-09 | KLOG-002 | **修订**：不拆双指标，用单指标 + tag：`log_count{level,event,drop}`，drop=0/1；接口保持单方法 `ReportLog(..., dropped bool)` | 王总：tag 方案更简单。drop=1 时 event 为空串（Enabled 处拿不到 event，结构性约束） |
| 2026-08-09 | KLOG-007 | **修订**：只做路线 a（immutable ctx-attrs），路线 b（Baggage）WONTFIX | 王总否决 Baggage |
| 2026-08-09 | KLOG-011 | **简化定稿**：runloop 是 daemon，每事件独立 root span，不做 span link 回投递方；`IEvent` 接口不动，改动仅 runloop.go 事件循环 | 王总：来源请求 id 不重要。副产品：事件作为 root span 走 sampler，daemon 与 HTTP 获得一致的采样提级机制 |
| 2026-08-09 | KLOG-011 | **两级身份映射定稿**：哪次 run → trace_id（每事件一个 trace，嵌套 span 不破坏分组）；哪个 runloop → ambient attr `runloop=<name>`（`Run()` 入口 `CtxWithAttrs` 一行，Phase 3 第 1 层的首个内部用户） | 曾讨论"每 runloop 一个 trace_id + 每 run 一个 span_id"方案，否决原因：①采样退化为每 runloop 一锤定音 ②嵌套 span 使 span_id 查询漏日志 ③长寿 trace 是未来接后端的地雷。attr 比 trace_id 更优：人类可读、跨重启稳定 |
| 2026-08-09 | KLOG-007 | **设计定稿（三层）**：① immutable attrs 链 + Importance 分级（保留，xklib 是库，不以本仓库用量判断取舍）；② `AttrProvider` 接口承接请求级可变对象（costCenter 类场景：对象自管并发，Handler 打日志时刻调 `LogAttrs()` 取最新快照）；③ `ModifyByKey` 移除（王总授权：并发风险大可删），其场景由 ①同key覆盖 / ②provider 接管 | 旧 CtxInfo 的病根 = 把"写一次读多次的分层字段"和"多模块并发读写的共享对象"塞进同一个带锁结构；拆开后两边都无锁负担。ctx 不只为 tracing——span/attrs/provider 是平行房客，公共约定只有 Handler 打日志时逐个收集 |

---

## 6. 本次对话补充结论

- CtxInfo 不只是"旧的 trace 传播"——它同时是 ambient 字段（R1）和跨异步边界的
  上下文继承机制。迁移时只显式接替了 R2/R3，R1 和"跨队列继承"是静默丢失的。
- "OpenTelemetry handles trace IDs automatically" 这句注释是迁移时的错误假设
  （只装了 propagator 没装 SDK，后台路径无 trace 可言），KLOG-011 由此而来。
