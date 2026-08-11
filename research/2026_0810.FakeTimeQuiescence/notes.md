# FakeTimeProvider 静默（quiescence）改造

日期：2026-08-10
分支：`feature/fake-time-quiescence`
起因：XklibSmellScan 收尾后，用户追问 `MockTimeProvider` 的设计意图，进而指出
`VirtualTimeForward` 跳钟前那次真实睡眠"is kind of a bandage"。

> 本文的决策日志由 2026-08-10 两个 session 的对话复原（前一个 session 在跑
> shardmgr 回归时因权限流中断，落盘这一步没轮到）。代码注释里能查证的部分标
> 了 file:line；只存在于对话里的取舍原样引用。

---

## 背景：kcommon 里有两代虚拟时钟

整个 `TimeProvider` 抽象的**全部回报**都在测试里——xklib/shardmgr 从不直接碰
`time.Now()`/`time.AfterFunc`，一律走 `kcommon.GetWallTimeMs()/ScheduleRun()/SleepMs()`，
这层间接换来的是"协议时间与墙钟时间解耦"。

- **MockTimeProvider**（`time.go:89`）——手摇木偶。`WallTime/MonoTime` 是裸字段，
  测试手工 `SetTimeMs/AddTimeMs`；`ScheduleRun` 只把任务塞进 `ChTask`（cap 10），
  由测试自己决定何时调 `Cb()`。无排序、无自动执行。使用方只剩 kcommon 自己的
  `timer_test.go`。
- **FakeTimeProvider**（`fake_time_provider.go`）——真正的主力，一台**离散事件仿真
  引擎**：最小堆当事件队列，`VirtualTimeForward` 是仿真主循环（塞哨兵任务 → 跑到期
  任务 → 没到期就把虚拟钟**瞬移**到堆顶到期时刻 → 级联调度 → 直到哨兵触发）。
  shardmgr 十几个 `assemble_*`/`sim_*` 测试全骑在它上面，56 个调用点。

**MockTimeProvider 去留（用户裁定）**：保留。"easy to understand, easy to use,
can function as a demo of TimeProvider interface"。不因本仓库只剩一个测试用户就
判死刑——使用者在仓库之外（xklib 哲学，见 CLAUDE.md 第 5 条）。

---

## FTQ-001 · 跳钟前用真实睡眠"猜"系统安静了 【病根 · 已修】

**旧实现**：跳钟前先睡一轮 100µs 真实时间（`sleepAtThisTime` 舞步），给并发的
runloop goroutine 留出处理事件、调度后续任务的机会。因为 FakeTimeProvider
观察不到 goroutine 状态，跳钟前那个必须回答的问题——**"有没有并发 goroutine 正在
干活、马上要往堆里塞新任务？"**——它只能睡一轮赌一把。

**Failure scenario（三条）**：

1. **正确性靠运气**：事件处理超过 100µs 真实时间（solver 大计算轻松超过）→ 虚拟钟
   提前跳 → 后续任务"迟到" → CI 机器一忙就 flaky。与 XklibSmellScan 那轮修的
   select 掷骰子同族。
2. **速度税**：每次跳钟至少付 100µs 墙钟；几千次跳的仿真付出真实秒级。
3. **逃生舱是创可贴上的创可贴**：`sleepCounter < 20` 放弃并返回 false，它存在本身
   就是"我分不清死锁和慢"的自白。
4. （顺带）**注释漂移**：注释写 "sleep counter reached 1000"，代码是 `< 20`。

---

## 方案对比与决策

### 方案 A · 自建静默计数器（采纳）

红利来自 actor 架构：**全部跨 goroutine 的工作只经过 xklib 自家两个咽喉**——
runloop 队列（Enqueue→Process）和 `ScheduleRun` 定时器（已在堆里）。所以静默是
可精确计数的：原子 in-flight 计数，Enqueue +1、Process 返回后 −1；跳钟条件从
"睡了一轮没变化"改成 `inFlight == 0 && 堆顶到期时刻 > 当前虚拟时`。

**如实的局限**：watcher 类 goroutine（从 fake etcd channel 收到事件、还没来得及
PostEvent 的瞬间）不在计数覆盖内。要堵严得让 fake etcd 投递侧也参与计数——可行，
但仪器化会蔓延，每个新异步源都要记得接入。**这是在手工重建运行时本来就知道的信息。**

### 方案 B · `testing/synctest`（推迟，non-goal for now）

已验货：本机 go1.26 里 `testing/synctest` 是正式标准库。核心语义一字不差就是要的
那个条件：

> Time in a bubble only advances when every goroutine in the bubble is
> **durably blocked**.

由 runtime 用地面真值回答"是否安静"：watcher 阻塞在 channel recv = 安静 ✓，
runloop 阻塞在 select = 安静 ✓，事件正在处理 = 可运行 → 时钟冻结 ✓。
**零仪器化，覆盖所有 goroutine，不只咽喉两点。** 深远含义：bubble 里
`SystemTimeProvider` 直接就是虚拟的，**FakeTimeProvider + TaskQueue +
VirtualTimeForward 整个 DES 引擎（155 行 + 那些微妙的舞步）理论上可以退役**，
`VirtualTimeForward(30_000)` 变回字面的 `time.Sleep(30*time.Second)`。

代价：go.mod 需从 1.21 升到 ≥1.25；bubble 纪律（所有 goroutine 须在 bubble 内启动、
不碰外部进程；包级 fire-and-forget goroutine 如 kmetrics 的日志需验证边界行为）；
`assemble_*` 装配 helper 迁移量不小。

**决策（用户，19:38）**：先做 A，B 是 non-goal for now。
**记录一笔分歧**：我当时的推荐是"B 是终局，A 是不想动工具链时的过渡"，并提议先花
一个 demo 的成本移植一个真实 sim 测试验货。用户选择不动工具链。B 未被否决，是被
推迟——本文留作它日重启的起点。

### 采纳后的形状（用户给的，19:38）

> using that atom counter as a signal, if still busy, sleep for another 100us
> and see again

本质：**把 100µs 从"承担正确性的赌注"降级为"轮询间隔"**——只影响发现静默的延迟，
不再影响对错。正确性全部转移到计数器这个精确条件上。条件驱动等待取代定时赌博。

---

## 实现中定下的四个边角

### FTQ-002 · in-flight 减数必须在 `Process` 之后 【已修】

`runloop.go:156`。若在 Process 之前减，handler 内 `ScheduleRun` 的后续任务还没入堆
计数就归零了 → 跳钟时任务堆不完整 → 时钟越过本该发生的工作。

### FTQ-003 · 停机配平：`enqMu` 【已修】

`unbounded_queue.go`。**Failure scenario**：queue 停止时 buffer/input 里仍有已 +1
但永不会被 Process 的事件 → 计数永久 >0 → 之后任何 `VirtualTimeForward` 永久冻结
（测试挂死）。修法：`Enqueue` 与关闭路径用 `enqMu` 互斥有序（临界区 = closed 检查
+ 一次 chan 发送，纳秒级），关闭路径持锁清点未消费事件批量补减。

**连带收益**：顺手消灭了 XklibSmellScan 遗留的"关闭竞态窗口内事件静默落入死 buffer"
缺陷——关闭后的 Enqueue 现在确定性走响亮丢弃路径。

### FTQ-004 · 保留一轮 grace sleep 【设计决定，非遗漏】

计数器覆盖不到 FTQ-001 里那个窄窗口（watcher 从 channel 收到数据到 PostEvent 之间）。
跳钟前仍睡一轮复核，作为对局限的诚实兜底。**它现在只是概率性加固，不是正确性的
承重结构**——承重的是计数器条件。彻底消掉它需要方案 B。

### FTQ-005 · 逃生舱静默 【本轮发现，已修】

**发现于**：改造完成后核对 56 个调用点时。

**问题**：`VirtualTimeForward` 的 doc 写着"false = 触发逃生舱，测试应视为失败信号"，
但全仓 56 个调用点**没有一个检查返回值**（旧实现的 `sleepCounter<20` 同样静默，
新版把逃生舱写进文档语义后缺口更刺眼）。

**Failure scenario**：真死锁 → 5s 后放弃 → false 被丢弃 → 虚拟钟停在半路 → 测试继续
跑 → 在后面某个断言上失败，报的是"shard 状态不对"之类的下游症状。调试的人要从错误
的地方倒推 5 秒钟的因果，真正的现场早已丢失。

**修法（三选二）**：
1. 改 56 个调用点检查返回值——侵入大，且测试作者会继续忘；
2. **采纳**：`slog.ErrorContext` + `panic(kerror)`，返回值随之删除（永远为 true 的
   bool 本身就是假信号）。provider 是 test-only 组件，仿真卡死本身就是测试 bug，
   就地 fail-fast 指向真现场。同形先例：`rand_util.go` 的 `CryptoRandSeedFailed`
   （KLOG-013③）。与"响亮丢弃 > 无声吞没"的既有取向同向。
3. 只加日志不 panic——可见但仍会产生误导性的下游失败。

两个逃生舱合并为单一出口 `giveUp()`，带现场信息（forwardMs / virtualTimeMs /
inFlightWork / pendingTasks）：
- `FakeTimeInFlightStuck`：计数持续不归零；
- `FakeTimeTaskQueueDrained`：堆异常清空（哨兵在堆里时不该出现）。

**签名变更（如实记录）**：`VirtualTimeForward(ctx, ms) bool` → 无返回值。不在
`TimeProvider` 接口内，仓内 56 个调用点全是语句式调用、零改动；仓外使用者会得到
编译错误（响亮，删掉 `if !` 即可）。

---

## 常量定价（自查 S4 fabricated-constant）

| 常量 | 值 | 性质 |
|---|---|---|
| `pollInterval` | 100µs | **不承担正确性**，只决定发现静默的延迟。沿用旧值，因为它已从赌注降级为轮询间隔。 |
| `maxBusyPolls` | 50000（×100µs = 5s） | **护栏，不是测量值**。正常事件处理是毫秒级，刻意取宽；只在真死锁时触发，宽一点只影响死锁测试的失败延迟。 |
| `maxEmptyPolls` | 20 | 沿用旧实现的阈值（旧注释误写作 1000，已改正）。同为护栏。 |

---

## 验证记录

- xklib 全包 `-count=1` ✅
- shardmgr 全包 `-count=1` ✅；`internal/core` `-count=5` ✅（52.8s / 5 轮 ≈ 10.6s
  每轮，与单跑一致，无偶发）
- cougar / unicorn / smgapp / etcdmgr / hellosvc / helloblitz `-count=1` ✅
- 新增测试：
  - `kcommon/inflight_test.go`：计数 >0 时虚拟钟冻结（真实时间过去 2ms 也不动）；
    死锁时 panic 且 kerror 类型正确；
  - `krunloop/inflight_accounting_test.go`：全生命周期归零；停机时 buffer 内事件
    补减、停机后掉队投递不污染计数。

---

## 决策日志

| # | 决策 | 理由 |
|---|---|---|
| D1 | 保留 MockTimeProvider | 易懂易用，可当 TimeProvider 接口的 demo；不以本仓库 grep 用量判死刑 |
| D2 | 走方案 A（自建计数器），B 推迟 | 不动工具链（go.mod 升级 + 全量测试迁移）；B 未被否决 |
| D3 | 计数器全局单例，不按 provider 类型分支 | 分支比白加一次原子操作贵；生产开销 = 每事件两次原子加，纳秒级 |
| D4 | 减数放在 `Process` 之后 | 保证 handler 内 `ScheduleRun` 先入堆（FTQ-002） |
| D5 | 用 `enqMu` 而非无锁配平 | 临界区纳秒级，换来计数零泄漏 + 消灭停机竞态窗口（FTQ-003） |
| D6 | 保留一轮 grace sleep | 兜住计数器覆盖不到的窄窗口；明确它不再承重（FTQ-004） |
| D7 | 逃生舱 panic，删除 bool 返回值 | 56 个调用点无人检查返回值，静默放弃会丢现场（FTQ-005） |

## 遗留

- **方案 B（`testing/synctest`）**：整个 DES 引擎的退役路径，本文留作重启起点。
- **grace sleep** 只能随 B 一起消掉。
- **fake etcd 投递侧未接入计数**：A 方案的已知局限，当前由 grace sleep 概率性兜底。
