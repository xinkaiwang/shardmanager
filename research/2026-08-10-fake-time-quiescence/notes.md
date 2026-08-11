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
2. **速度税**：每次跳钟至少付 100µs 墙钟。
   （**这条当时被我严重低估**——后来实测是 shardmgr `internal/core` 73% 的墙钟
   时间，一轮约 7 万次跳钟，不是"附带项"。见 FTQ-006。）
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

## FTQ-006 · 100µs 常量的实测定价与下调 【本轮，已改】

**起因**：用户问"既然有了 busy 标志，是不是可以把 100µs 降到 10µs 让测试更快"。

**先纠正 FTQ-001 里我自己的低估**：那里把"速度税"列为后果 #2，像个附带项。
实测它是 shardmgr `internal/core` **73% 的墙钟时间**。

### 实测（A/B/C 三臂，每臂 3 次，shardmgr internal/core）

| 臂 | poll | grace | 用时 |
|---|---|---|---|
| A 基线 | 100µs | 100µs | **10.6s** |
| B 全缩 | 10µs | 10µs | **2.2s**（4.8x） |
| C 只缩 poll | 10µs | 100µs | **9.9s**（1.07x） |

分解：busy 轮询只占 **0.7s（6%）**，grace sleep 占 **7.7s（73%）**。
量级推算：core 一轮约 **7 万次跳钟**，每次无条件付一个 grace。

### 关键认识：一个常量身兼两职，后果不同

- **pollInterval**（busy 轮询）：纯延迟，正确性中立 → 随便缩；
- **graceInterval**（跳钟前复核）：FTQ-004 那个**概率性正确性兜底** → 缩它 = 把
  刚用计数器买回来的性质拿去付账。

故拆成两个常量。名字不同、注释不同、调整后果不同。

### 决策（D8）：两个都降到 10µs，明知代价

我的建议是**不缩 grace，而是删掉它**——把 watcher 交棒（fake etcd 投递侧 +1、
PostEvent 后 −1）接进计数器，窗口封死后 grace 无存在理由，收益全拿（约 1s 量级，
比原来快 10 倍）且条件从"赌"变"确知"。
用户选择直接下调两个常量：一行改动拿 4.8 倍，兜底变薄的风险由他承担。

**如实记录我反对的理由**：grace 从实测 130µs 降到 18µs，兜底窗口薄到 1/7；而 CI
忙时正是窗口最宽的时候，也正是 FTQ-001 那个病复发的条件。

### 对该风险的证伪尝试（不是空载绿了就算数）

28 核机器起 56 个 CPU burner，load average 冲到 **30.1**，单轮从 2.1s 拖慢到 8.5s
（竞争确实发生），`internal/core` 连跑通过。详见验证记录。这削弱了我上面的担忧，
但**没有证明它不存在**——失效是概率性的，跑几十轮不构成证明，只是没抓到。

### 连带必改：护栏常量必须改用时间表达

`maxBusyPolls = 50000` 的真实语义是 "50000 × 100µs = 5s"。轮询间隔一改，护栏长度
跟着变而没人会想起来改它——按 10µs 算只剩约 0.9s，**一个算得久的 solver 事件会被
误判成死锁并 panic**。改为 `busyTimeout = 5s` / `emptyTimeout = 2ms`，用时刻差判断，
护栏长度不再随轮询间隔漂移。这是"次数表达时间"这一类坏常量的标准修法。

---

## 常量定价（自查 S4 fabricated-constant）

| 常量 | 值 | 性质 |
|---|---|---|
| `pollInterval` | 10µs | **不承担正确性**，只决定发现静默的延迟。实测请求 10µs 实际交付约 18µs。 |
| `graceInterval` | 10µs | **承担概率性正确性**（FTQ-004 的兜底）。取值是 D8 的自觉取舍：换 4.8 倍测试速度。 |
| `busyTimeout` | 5s | **护栏，不是测量值**。正常事件处理毫秒级，刻意取宽；只在真死锁时触发。 |
| `emptyTimeout` | 2ms | 旧实现 20 × 100µs 的等效值，同为护栏。 |

**测量方法的自我批评**：过程中我另外量了 `time.Sleep` 的实际交付时长（10µs→18µs、
100µs→132µs、200µs→259µs）。它只有一个正当角色——先确认"请求 10µs 能不能真睡出
10µs"，否则旋钮根本不存在。**拿到端到端 A/B/C 之后它就不再承重**，我却还拿它做了
一个换算模型和证伪臂（grace=200µs → 21.3s）。而且两条路径推出的跳钟次数差 25%
（67k vs 84k），因为单价是空载测的、不适用于测试跑起来时的调度环境。教训：端到端
测量一旦到手，中间单价就该退场，不要用弱工具去装点已经足够的结论。

---

## 验证记录

### 改造本体（FTQ-001..005，grace/poll 仍为 100µs）

- xklib 全包 `-count=1` ✅
- shardmgr 全包 `-count=1` ✅；`internal/core` `-count=5` ✅（52.8s / 5 轮 ≈ 10.6s
  每轮，与单跑一致，无偶发）
- cougar / unicorn / smgapp / etcdmgr / hellosvc / helloblitz `-count=1` ✅
- 新增测试：
  - `kcommon/inflight_test.go`：计数 >0 时虚拟钟冻结（真实时间过去 2ms 也不动）；
    死锁时 panic 且 kerror 类型正确；
  - `krunloop/inflight_accounting_test.go`：全生命周期归零；停机时 buffer 内事件
    补减、停机后掉队投递不污染计数。

### 常量下调（FTQ-006，grace/poll = 10µs）

- xklib 全包 `-count=1` ✅（含 `go vet ./...`）
- shardmgr 全包 `-count=1` ✅；`internal/core` **2.99s**（原 10.6s）；
  `-count=5` ✅ 9.77s（≈1.95s/轮）
- **负载下证伪尝试**：28 核机器起 56 个 `yes` burner，load average 冲到 **57**，
  `internal/core` 单轮从 2.1s 拖慢到 8.6s（竞争确实发生），`-count=10` **全绿**。
  另一轮 load 30 下 `-count=3` 全绿。
  **这削弱但没有证伪 D8 的风险**：失效是概率性的，没抓到不等于不存在。
- 逃生舱护栏改时间表达后，`TestVirtualTimeForward_BusyDeadlockPanics` 仍在 ~5s
  触发 ✅（护栏长度不再随轮询间隔漂移）。
- 另修一处边角：堆"空 → 有任务 → 再空"时 `emptySince` 未清零，两次短暂的空可能
  凑满 `emptyTimeout` 误触发逃生舱；改为堆不空即清零。

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
| D8 | poll 与 grace 拆成两个常量，均降到 10µs | 实测 4.8 倍提速；**我建议的是删掉 grace（封死 watcher 窗口）而非缩它，用户选择直接下调并承担兜底变薄的风险**（FTQ-006） |
| D9 | 护栏改用时间表达（`busyTimeout`/`emptyTimeout`） | 次数×间隔表达的时间会随间隔漂移；按 10µs 算旧护栏只剩 0.9s，会把慢 solver 误判成死锁（FTQ-006） |

## 遗留

- **方案 B（`testing/synctest`）**：整个 DES 引擎的退役路径，本文留作重启起点。
- **grace sleep 仍在**，且现在只有 18µs 实际厚度。彻底了结有两条路：
  ① 把 watcher 交棒接进 in-flight 计数（窗口封死，grace 可删，约 1s 量级）；
  ② 方案 B（连计数器一起退役）。
- **fake etcd 投递侧未接入计数**：A 方案的已知局限，当前由更薄的 grace 概率性兜底。
