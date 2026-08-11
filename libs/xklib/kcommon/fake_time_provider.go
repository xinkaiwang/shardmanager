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

// pollInterval 是静默轮询间隔：只决定"发现系统变安静的延迟"，不承担任何
// 正确性（正确性来自 InFlightWorkCount 条件）。
const pollInterval = 100 * time.Microsecond

// maxBusyPolls 是深度死锁逃生舱：in-flight 计数持续不归零（事件处理真死锁）
// 时放弃等待，让测试以失败而非挂起的方式暴露问题。
// 取值 50000 × 100µs = 5s 真实时间——刻意取宽（正常事件处理是毫秒级），
// 它是护栏不是测量值，只在真死锁时触发，宽一点只影响死锁测试的失败延迟。
const maxBusyPolls = 50000

// maxEmptyPolls 是任务堆异常清空的逃生舱：本次推进的哨兵任务一直在堆里，
// 堆不该为空；连续 20 轮为空说明有别的调用方把堆抽干了（如任务回调里嵌套
// 调用 VirtualTimeForward 顺手跑掉了本次的哨兵），仿真无法到达目标时刻。
const maxEmptyPolls = 20

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

	emptyPolls := 0
	busyPolls := 0
	graceDone := false // 每次跳钟决定前的一轮复核标记
	for !vtDeadline {
		// 第一关：in-flight 工作未清零 → 时钟冻结
		if InFlightWorkCount() > 0 {
			busyPolls++
			if busyPolls >= maxBusyPolls {
				provider.giveUp(ctx, "FakeTimeInFlightStuck",
					"virtual clock frozen: in-flight work never drained (event processing deadlock?)", forwardMs)
			}
			graceDone = false
			time.Sleep(pollInterval)
			continue
		}
		busyPolls = 0

		var needRunTask *FakeTimerTask
		needSleep := false
		RunWithLock(&provider.mu, func() {
			topTask := provider.taskQueue.Peek()
			if topTask == nil {
				needSleep = true
				emptyPolls++
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
		if needSleep {
			if emptyPolls >= maxEmptyPolls {
				provider.giveUp(ctx, "FakeTimeTaskQueueDrained",
					"virtual clock stalled: task heap empty although this call's sentinel should be in it", forwardMs)
			}
			time.Sleep(pollInterval)
			continue
		}
		if needRunTask != nil {
			emptyPolls = 0
			needRunTask.TaskFunc()
			continue
		}
	}
}
