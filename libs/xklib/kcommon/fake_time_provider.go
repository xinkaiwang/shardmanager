package kcommon

import (
	"container/heap"
	"sync"
	"time"
)

// FakeTimeProvider: implements TimeProvider interface
type FakeTimeProvider struct {
	WallTime int64
	MonoTime int64

	taskQueue *TaskQueue
	mu        sync.Mutex
}

func NewFakeTimeProvider() *FakeTimeProvider {
	return &FakeTimeProvider{
		taskQueue: &TaskQueue{},
	}
}

func (provider *FakeTimeProvider) GetWallTimeMs() int64 {
	return provider.WallTime
}

func (provider *FakeTimeProvider) GetMonoTimeMs() int64 {
	return provider.MonoTime
}

func (provider *FakeTimeProvider) ScheduleRun(delayMs int, fn func()) {
	task := &FakeTimerTask{
		TaskFunc:   fn,
		MonoTimeMs: provider.GetMonoTimeMs() + int64(delayMs),
	}
	RunWithLock(&provider.mu, func() {
		heap.Push(provider.taskQueue, task)
	})
}

func (provider *FakeTimeProvider) SimulateForward(forwardMs int) {
	reachedDeadline := false
	provider.ScheduleRun(forwardMs, func() {
		reachedDeadline = true
	})

	// empty job to cause 1ms sleep to allow runloop processing
	provider.ScheduleRun(0, func() {})
	for !reachedDeadline {
		var task *FakeTimerTask
		RunWithLock(&provider.mu, func() {
			task = heap.Pop(provider.taskQueue).(*FakeTimerTask)
		})

		provider.MonoTime = task.MonoTimeMs
		provider.WallTime = task.MonoTimeMs
		task.TaskFunc()
		time.Sleep(time.Millisecond * 1)
	}
}
