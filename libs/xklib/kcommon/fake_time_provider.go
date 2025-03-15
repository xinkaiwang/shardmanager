package kcommon

import (
	"container/heap"
	"context"
	"sync"
	"time"

	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
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
	return provider.WallTime
}

func (provider *FakeTimeProvider) GetMonoTimeMs() int64 {
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

func (provider *FakeTimeProvider) VirtualTimeForward(ctx context.Context, forwardMs int) bool { // return true if vt deadline is successfully reached. false means sleep counter reached 1000 before vt deadline (to avoid infinite deadlock in test case)
	currentVt := provider.GetMonoTimeMs()
	sleepCounter := 0
	vtDeadline := false
	provider.ScheduleRun(forwardMs, func() {
		vtDeadline = true
	})

	// empty job to cause 1ms sleep to allow runloop processing
	// provider.ScheduleRun(0, func() {})
	sleepAtThisTime := false
	for !vtDeadline && sleepCounter < 20 {
		var needRunTask *FakeTimerTask
		needSleep := false
		RunWithLock(&provider.mu, func() {
			topTask := provider.taskQueue.Peek()
			entry := klogging.Info(ctx).With("topTask", topTask).With("currentVt", currentVt).With("vtDeadline", vtDeadline).With("sleepCounter", sleepCounter).With("sleepAtThisTime", sleepAtThisTime)
			defer func() {
				entry.With("needSleep", needSleep).With("needRunTask", needRunTask).Log("SimulateForward", "topItem")
			}()
			if topTask == nil {
				needSleep = true
				sleepCounter++
				return
			}
			if topTask.ScheduledForMs <= currentVt {
				needRunTask = topTask
				heap.Pop(provider.taskQueue)
				return
			}
			if !sleepAtThisTime {
				needSleep = true
				sleepAtThisTime = true
				return
			} else {
				currentVt = topTask.ScheduledForMs
				provider.MonoTime = currentVt
				provider.WallTime = currentVt
				sleepAtThisTime = false
				needRunTask = topTask
				heap.Pop(provider.taskQueue)
			}
		})
		if needSleep {
			time.Sleep(time.Millisecond * 1)
			continue
		}
		if needRunTask != nil {
			needRunTask.TaskFunc()
			continue
		}
	}
	return vtDeadline
}
