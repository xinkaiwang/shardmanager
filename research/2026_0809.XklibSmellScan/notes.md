# xklib 全库 smell 扫描

**日期**: 2026-08-09
**动机**: xklib 是所有服务的地基，做一次系统性质量体检。
**规则**: 每个 finding 必须带 failure scenario（无后果链只能记"观察"）；编号 XS-NNN。
**扫描清单**: held-ctx / twin-paths+layer-collapse / silent-swallow / S14 trace-blind /
S4 fabricated-constant / S2 confirmation-only。（S1/S3/S15 等业务侧模式不适用，跳过。）

---

## 扫描 1：held-ctx — ✅ 干净

全库唯一 held ctx = `RunLoop.ctx`（runloop.go:54）：
- Question A：真 runloop（Run 的 select 听 `<-rl.ctx.Done()`），字段承重；
- Question B：外部入口无泄漏（PostEvent 无 ctx 副作用；getter 纯读；
  Run/InitTimeSeries 收调用方 ctx；StopAndWaitForExit 属自我生命周期豁免）。

## 扫描 2：half-fact-twin-paths + semantic-layer-collapse — 2 命中

### XS-001 · `rl.stop` 死掉的孪生触发器 【P3 · OPEN】
- runloop.go:70 创建、:151 select 监听、全库无人 close。
- Failure scenario：运行时永不触发；代价是读者被骗——select 三分支暗示存在第三种
  停止协议；将来有人外部 `close(rl.stop)` "激活"它时会绕过 StopAndWaitForExit 的
  queue-first 顺序（queue 不关、Enqueue 不拒），退化成 XS-002 场景。
- 修法：删字段 + 删 select 分支。

### XS-002 · "Run 退出" ≠ "runloop 停止"，queue 半边靠隐式约定 【P2 · OPEN】
- queue 听构造时的 ctx（unbounded_queue.go:88），Run 听自己参数派生的 ctx；
  两者一致性无人保证（API 允许 NewRunLoop(ctxA) + Run(ctxB)）。
- Failure scenario：cancel ctxB → Run 退出，queue goroutine 存活、closed 未置位，
  PostEvent 继续无 panic 入 buffer、无人消费 → 无声无界内存增长，零信号。
- 修法：Run 的 defer 加权威收尾 `rl.queue.Stop()`（结构保证取代约定）；
  queue.Stop 需 sync.Once 幂等化（现为裸 close，双路会 double-close panic）。
- layer-collapse 检查：krunloop 为库级循环，无业务宣告层，teardown 三层合一合法，无 collapse。

## 扫描 3：silent-swallow — 2 命中 + 1 观察

### XS-003 · ksysmetrics 十连 `_ =` 吞 gauge 注册错误，nil 崩在别处 【P2 · OPEN】
- sysmetrics.go:87-153 共 10 处 `xxxGauge, _ = registry.AddXxxDerivedGauge(...)`，
  紧接 `xxxGauge.UpsertEntry(...)`。
- Failure scenario：初始化被调两次 / 某服务注册了同名指标 → Add 返回 err + nil gauge →
  `UpsertEntry` nil 解引用 panic，栈指向 UpsertEntry 而非真正病因（名字冲突）——
  与 kcommon rand 旧 bug 同族（吞错处与死处分离）。
- 修法：仿同库 kmetrics/gauge.go:72 的现成好范式——err 时 panic
  `kerror("sysGaugeRegisterFail").With("gaugeName", ...)`，响亮且指向病因。
  （sibling convention 已存在，属 foundation-amnesia 的反向修复。）

### XS-004 · decorator.go:31 panic 路径上的 `fmt.Printf` 【P3 · OPEN】
- `fmt.Printf("panic type: %T\n", r)` —— panic 现场最宝贵的类型信息走了
  stdout 裸通道：无结构、无 event、无 trace 关联，Splunk 无法检索。
- Failure scenario：生产环境 panic 被 decorator 捕获转 kerror，排障者在
  结构化日志里找不到 panic 的原始类型（信息在 stdout 混流里）。
- 修法：删 printf，把 `%T` 塞进 kerror detail（`With("panicType", fmt.Sprintf("%T", r))`）。

### 观察（不赋级）
- tracing.go:65 `_ = tp.Shutdown(ctx)`：进程退出时的 best-effort，no-export 模式下
  Shutdown 无实义失败，注释已声明——有意吞，合规。
- 清白参照：kmetrics/gauge.go（err 全部响亮 panic + 带 gaugeName）、
  sysmetrics 采集循环（Getrusage/getFDCount 的 err 分支都有带 error 的 slog）。

## 扫描 4：S14 trace-blind-log — ✅ 干净
- 裸 `slog.X()`（非 Context 变体）：**0 处**——全库遵守 ctx 契约。
- `context.Background()` 6 处分桶：registry.go:152/166（包 init/main 期注册冲突→exit，
  无 ctx 存在，Bucket D）、sysmetrics.go:56（进程级单例检查，D）、hello.go:57（demo，D）、
  runloop.go:179 + unbounded_queue.go:141（StopAndWaitForExit 无 ctx 参数——被
  KLOG-009 WONTFIX 锁定，Bucket E 维持原判）。

## 扫描 5：S4 fabricated-constant — 1 TP + 2 suspect + 1 意外重磅

### XS-005 · RunloopSampler 永不停止（S4 扫描中的意外发现，twin-paths 家族）【P2 · OPEN】
- sampler.go:37 `kcommon.ScheduleRun(20, func(){ rs.Run(ctx) })` 无条件自我重调度；
  全函数无 ctx.Done 检查、无 stop 字段、无 Stop 方法——**"停止 runloop"这个事件
  又缺了一半**（XS-002 的姊妹：queue 有隐式协调，sampler 干脆没有协调）。
- Failure scenario：每创建一个 RunLoop 就泄漏一条永久的 50Hz 定时器链
  （AfterFunc→Run→AfterFunc…）+ 每 20ms 一次 metric 写入（带停用 runloop 的 name tag）。
  测试套件每跑一个 runloop 测试漏一条；生产若有 runloop 动态创建/销毁（按 shard 建
  loop 之类），定时器无界累积。当前各服务 runloop 均进程级长命，故未爆发——
  属"结构性泄漏，等待第一个动态场景引爆"。
- 修法：Run 每 tick 先查 `ctx.Err() != nil → return`（一行，靠 ctx 终止链条）；
  RunLoop.StopAndWaitForExit 的 cancel() 恰好能触发它——前提是 XS-002 修复后
  ctx 语义统一。

### XS-006 · StopAndWaitForExit 的 1000ms 是编造常量 【P3 · OPEN】
- runloop.go:177 `time.After(1000 * time.Millisecond)` + 注释"增加超时时间，
  **确保有足够时间**退出"——hedge 词 + 整数圆 + 零出处，且**承重**：它决定
  停机时最多等事件处理多久。
- Failure scenario：shutdown 时某事件（如 solver 大计算、etcd 慢调用）处理超 1s →
  StopAndWaitForExit 放弃等待返回 → 调用方继续拆除事件正在使用的资源 →
  停机路径 use-after-teardown 竞态；且只有一条 Warn 日志，无人会当回事。
- 修法方向（不替号，按 S4 铁律）：用已有的 `runloop_elapsed_ms` 指标实测
  事件时长分布来定值；或改为可配置；或无限等待 + 周期性进度日志。
  （美妙的是：接地它所需的测量指标**本来就存在**。）

### Suspect（plausible-but-uncited，低风险）
- sampler.go:37 采样周期 20ms（"=50Hz"只是换算不是出处）；成本低（每 tick 一次
  atomic 读+加），不承重，建议注释补一句选值理由即可。
- kcommon/time.go:98 MockTimeProvider `ChTask` 容量 10：测试排 >10 个定时任务不 drain
  会死锁，症状迷惑。测试基建，低风险。

### FP（已验出处/不承重）
- rand buf 8 字节 = int64 定义推导；kerror `Grow(1000)` 非承重预分配；
  queue `make(chan,1)` 有设计意图注释——但注释"**ensure** Enqueue doesn't block"
  过度承诺（并发第二个 Enqueue 在消费者忙时仍会短暂阻塞），措辞该收敛（观察）。

### 顺带发现（转交 S2）
- krunloop 三个 metric 的 description 全是占位符字符串 `"desc"`
  （runloop.go:17,18、sampler.go:11）——operator 面元数据是垃圾值。

## 扫描 6：S2 confirmation-only — 2 命中（krunloop）

### XS-007 · UnboundedQueue：无深度 gauge、无入队计数——in/out/drop 三角缺 in 【P1 · OPEN · 子模式 F+E】
- queue 是标准数据流组件。现有指标只覆盖 out（`runloop_elapsed_ms`/`runloop_queue_time_ms`
  的 count = 已处理事件数）；**in（入队速率）与 depth（当前积压）完全没有指标**。
- 讽刺证据：`GetQueueLength()` 方法存在——作者想到了这个问题，但只做成了
  给自己调试的 API，没接到 operator 面（builder-eye 的标准形态）。
- Failure scenario：某类事件处理变慢（etcd 抖动/solver 大计算）→ 生产者继续
  PostEvent → **无界队列**内存线性增长 → Grafana 上零预警指标，operator 的
  第一个信号是 OOM kill。事后想查"几点开始积压的"也无数据。
- 操作员问不出答案的问题："现在队列积压多少？入队速率 vs 处理速率差多少？"
- 修法：入队计数 metric（in）+ 深度 gauge（或由 in−out 推导；gauge 更直接）。

### XS-008 · 三个 metric 的 description 是占位符 "desc" 【P3 · OPEN · 子模式 D 反向】
- runloop.go:16,17（elapsed_ms / queue_time_ms）、sampler.go:11（sample_ct）。
- Grafana tooltip 里 operator 看到的就是字面 "desc"。
- 修法：各补一句 operator 可读的真描述。
- 清白参照：Phase 2 的 `log_size{level,event,drop}` 自带分母（drop tag 自配对）✓。

---

## 总结

| 编号 | 问题 | 级 | 包 |
|---|---|---|---|
| XS-001 | rl.stop 死通道 | P3 | krunloop |
| XS-002 | Run 退出不停 queue（隐式 ctx 约定） | P2 | krunloop |
| XS-003 | sysmetrics 十连吞 gauge 注册错误→nil 崩别处 | P2 | ksysmetrics |
| XS-004 | decorator panic 路径 fmt.Printf 裸通道 | P3 | kmetrics |
| XS-005 | RunloopSampler 永不停止（50Hz 定时器链泄漏） | P2 | krunloop |
| XS-006 | StopAndWaitForExit 1000ms 编造常量+放弃语义 | P3 | krunloop |
| XS-007 | queue 无深度/入队指标，OOM 前零预警 | P1 | krunloop |
| XS-008 | metric description = "desc" ×3 | P3 | krunloop |

干净项：held-ctx（RunLoop.ctx 合规）、S14（零裸 slog，Background 6 处全豁免）、
kerror/klogging/kcommon 主体（多处清白参照）。

## 修复记录（2026-08-10，分支 fix/xklib-smell-scan）

- **XS-001..008 全部修复**：xklib 一笔（`a91307c`）+ shardmgr 一笔（`8c6eb3a`），
  全程 TDD，全仓 -count=1 回归全绿（core 连跑 5 次稳定）。
- **XS-003 改判**：P2→P3。初判 failure scenario（"init 被调两次/同名注册"）不成立——
  gauge 全在 init() 注册且 registry 同函数新建，语言保证不可重入。存活后果链仅剩
  "未来复制粘贴忘改名"。修复中另发现：opencensus 对同名同类型重复注册**不报错而是
  静默孤儿化旧 gauge**（kmetrics/gauge.go 注释早有记载），故 helper 自建名字查重。
- **Enqueue 后停机语义两次迭代**（决策记录）：
  1. 初版 panic（fail-fast）→ 被真实代码推翻：shardmgr 的延迟定时器回调
     （AcceptEvent 重试等）是**合法的后停机投递者**，与 shutdown 赛跑输了不是 bug，
     panic 会把优雅停机窗口的良性掉队者变成崩溃，而给全部定时投递加防护是
     大范围侵入且本质有竞态。
  2. 终版：**响亮丢弃**——Warn（带事件名）+ `runloop_enqueue_dropped_ct`，
     不阻塞不崩溃。可见性不降，顺手补齐 XS-007 三角的 drop 边。
- **Loud 语义照出两个 shardmgr 存量 bug**（修复的连带战果）：
  - ServiceState 拆除顺序反了（先停消费者 runloop 后停生产者 watchers）→
    反转为数据流序；
  - ShardPlanWatcher "ch closed" 分支裸 return 跳过 close(stopped) → 停机永久挂死
    （偶发，时序侥幸掩盖多年）；WorkerEphWatcher 根本不检查 ch 关闭 → 关闭后
    忙循环解析零值。均为 half-fact twin path 在业务侧的实例。
- 未修（记录在案）：sampler 20ms/MockTimeProvider cap 10（suspect 不承重）、
  queue buffer=1 注释措辞、GetSize 停机后残值（无消费者）。

---

## 修复记录

（扫描全部完成后统一修复，TDD，单独分支。）
