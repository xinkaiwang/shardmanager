package kcommon

import (
	"container/heap"
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
)

// FakeTimeProvider: implements TimeProvider interface
type FakeTimeProvider struct {
	WallTime int64
	MonoTime int64

	taskQueue *TaskQueue
	mu        sync.Mutex
}

func NewFakeTimeProvider(currentTime int64) *FakeTimeProvider {
	return &FakeTimeProvider{
		WallTime:  currentTime,
		MonoTime:  currentTime,
		taskQueue: NewTaskQueue(),
	}
}

func (provider *FakeTimeProvider) GetWallTimeMs() int64 {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	return provider.WallTime
}

func (provider *FakeTimeProvider) GetMonoTimeMs() int64 {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	return provider.MonoTime
}

func (provider *FakeTimeProvider) SleepMs(ctx context.Context, ms int) {
	provider.VirtualTimeForward(ctx, ms)
}

func (provider *FakeTimeProvider) ScheduleRun(delayMs int, fn func()) {
	task := &FakeTimerTask{
		TaskFunc:       fn,
		ScheduledForMs: provider.GetMonoTimeMs() + int64(delayMs),
	}
	RunWithLock(&provider.mu, func() {
		heap.Push(provider.taskQueue, task)
	})
}

// pollInterval 是 busy 轮询间隔：只决定"发现系统变安静的延迟"，不承担任何
// 正确性（正确性来自 InFlightWorkCount 条件）。
const pollInterval = 10 * time.Microsecond

// graceInterval 是跳钟前那一轮复核的时长——与 pollInterval 语义不同：它是
// 概率性的正确性兜底，覆盖 in-flight 计数看不见的窄窗口（watcher 从 channel
// 收到数据到 PostEvent 之间）。两者曾共用一个常量，2026-08-10 拆开，因为
// 调它们的后果完全不同：pollInterval 只影响延迟，graceInterval 影响对错概率。
//
// 定价（实测，2026-08-10）：grace 是仿真的主要成本——shardmgr internal/core
// 一轮约 7 万次跳钟，每次无条件付一个 grace。实测 core 用时：
// grace=100µs → 10.6s，grace=10µs → 2.2s（只降 pollInterval 则是 9.9s，
// 即 busy 轮询仅占 6%）。取 10µs 是**用兜底厚度换 4.8 倍测试速度的自觉取舍**，
// 决策与反对意见见 research/2026-08-10-fake-time-quiescence/notes.md 的 D8。
const graceInterval = 10 * time.Microsecond

// busyTimeout 是深度死锁逃生舱：in-flight 计数持续不归零（事件处理真死锁）
// 时放弃等待，让测试以失败而非挂起的方式暴露问题。
//
// 用时间而不是轮询次数表达（2026-08-10）：旧版写作 maxBusyPolls=50000，其真实
// 语义是 "50000 × 100µs = 5s"——轮询间隔一变，护栏长度就跟着变，而没人会想到
// 去改它。按 10µs 算旧常量只剩约 0.9s，一个算得久的 solver 事件就会被误判成
// 死锁并 panic。5s 是护栏不是测量值：正常事件处理是毫秒级，刻意取宽，只在真
// 死锁时触发，宽一点只影响死锁测试的失败延迟。
const busyTimeout = 5 * time.Second

// emptyTimeout 是任务堆异常清空的逃生舱：本次推进的哨兵任务一直在堆里，堆不该
// 为空；持续为空说明有别的调用方把堆抽干了（如任务回调里嵌套调用
// VirtualTimeForward 顺手跑掉了本次的哨兵），仿真无法到达目标时刻。
// 2ms 沿用旧实现的等效值（旧版 20 × 100µs），同样按时间表达。
const emptyTimeout = 2 * time.Millisecond

// giveUp 是仿真引擎放弃推进时的唯一出口——响亮失败。
//
// 为什么是 panic 而不是返回 false（2026-08-10）：旧签名返回 bool 表示
// "是否到达目标时刻"，但全仓 56 个调用点没有一个检查返回值。放弃推进于是
// 变成静默事件：虚拟钟停在半路，测试继续跑，最终在几秒后的某个下游断言上
// 失败（"shard 状态不对"），而真正的现场——时钟没推进到位——已经丢了。
// FakeTimeProvider 是 test-only 组件，仿真卡死本身就是测试 bug，就地
// fail-fast 指向真现场。同形先例：rand_util.go 的 CryptoRandSeedFailed。
func (provider *FakeTimeProvider) giveUp(ctx context.Context, errType, msg string, forwardMs int) {
	var pending int
	var vt int64
	RunWithLock(&provider.mu, func() {
		pending = provider.taskQueue.Len()
		vt = provider.MonoTime
	})
	inFlight := InFlightWorkCount()
	slog.ErrorContext(ctx, msg,
		slog.String("event", errType),
		slog.Int("forwardMs", forwardMs),
		slog.Int64("virtualTimeMs", vt),
		slog.Int64("inFlightWork", inFlight),
		slog.Int("pendingTasks", pending))
	panic(kerror.Create(errType, msg).
		With("forwardMs", forwardMs).
		With("virtualTimeMs", vt).
		With("inFlightWork", inFlight).
		With("pendingTasks", pending))
}

// VirtualTimeForward 推进虚拟时钟 forwardMs 毫秒，途中按到期顺序执行任务堆
// 中的任务。正常返回 = 已到达目标时刻；无法到达则 panic（见 giveUp）。
//
// 跳钟纪律（2026-08-10 重写，旧版靠"睡一轮赌安静"，CI 忙时会提前跳钟）：
//  1. InFlightWorkCount() > 0（有事件已入队未处理完）→ 冻结虚拟钟，轮询等待；
//  2. 计数归零后、真正跳钟前，仍保留一轮 grace sleep 复核——保护不在计数
//     覆盖内的窄窗口（如 watcher 从 channel 收到数据到 PostEvent 之间）；
//  3. 两关都过才把时钟跳到下一个任务的到期时刻。
func (provider *FakeTimeProvider) VirtualTimeForward(ctx context.Context, forwardMs int) {
	vtDeadline := false
	provider.ScheduleRun(forwardMs, func() {
		vtDeadline = true
	})

	// 两个逃生舱都用"起始时刻 + 超时"表达，零值 = 当前不在该状态里
	var busySince, emptySince time.Time
	graceDone := false // 每次跳钟决定前的一轮复核标记
	for !vtDeadline {
		// 第一关：in-flight 工作未清零 → 时钟冻结
		if InFlightWorkCount() > 0 {
			switch {
			case busySince.IsZero():
				busySince = time.Now()
			case time.Since(busySince) >= busyTimeout:
				provider.giveUp(ctx, "FakeTimeInFlightStuck",
					"virtual clock frozen: in-flight work never drained (event processing deadlock?)", forwardMs)
			}
			graceDone = false
			time.Sleep(pollInterval)
			continue
		}
		busySince = time.Time{}

		var needRunTask *FakeTimerTask
		needSleep := false
		sleepDur := pollInterval
		emptyHeap := false
		RunWithLock(&provider.mu, func() {
			topTask := provider.taskQueue.Peek()
			if topTask == nil {
				needSleep = true
				emptyHeap = true
				return
			}
			if topTask.ScheduledForMs <= provider.MonoTime {
				needRunTask = topTask
				heap.Pop(provider.taskQueue)
				return
			}
			// 第二关：跳钟前保留一轮 grace 复核（睡一轮后重查计数与堆）
			if !graceDone {
				needSleep = true
				sleepDur = graceInterval
				graceDone = true
				return
			}
			// 两关都过：跳钟到下一个到期时刻
			provider.MonoTime = topTask.ScheduledForMs
			provider.WallTime = topTask.ScheduledForMs
			graceDone = false
			needRunTask = topTask
			heap.Pop(provider.taskQueue)
		})
		// 堆一旦不空就清零计时——否则"空 → 有任务 → 再空"会沿用上一次的起点，
		// 两次短暂的空加起来可能凑满 emptyTimeout，误触发逃生舱
		if !emptyHeap {
			emptySince = time.Time{}
		}
		if needSleep {
			if emptyHeap {
				switch {
				case emptySince.IsZero():
					emptySince = time.Now()
				case time.Since(emptySince) >= emptyTimeout:
					provider.giveUp(ctx, "FakeTimeTaskQueueDrained",
						"virtual clock stalled: task heap empty although this call's sentinel should be in it", forwardMs)
				}
			}
			time.Sleep(sleepDur)
			continue
		}
		if needRunTask != nil {
			needRunTask.TaskFunc()
			continue
		}
	}
}
