package krunloop

import (
	"context"
	"testing"
)

// captureScheduler 收集被调度的回调，测试手工驱动 tick——零全局状态、完全确定性
type captureScheduler struct{ tasks []func() }

func (cs *captureScheduler) schedule(delayMs int, fn func()) { cs.tasks = append(cs.tasks, fn) }

// XS-005：Stop() 后 sampler 不得再自我重调度
func TestRunloopSampler_StopTerminatesRescheduleChain(t *testing.T) {
	cs := &captureScheduler{}
	rs := &RunloopSampler{name: "xs005", fn: func() string { return "e" }, scheduleFn: cs.schedule}

	rs.Run(context.Background())
	if len(cs.tasks) != 1 {
		t.Fatalf("first tick should schedule exactly one follow-up, got %d", len(cs.tasks))
	}

	rs.Stop()
	cs.tasks[0]() // 手工触发下一 tick——应立即返回，不再调度
	if len(cs.tasks) != 1 {
		t.Fatalf("sampler rescheduled after Stop() (XS-005): %d tasks", len(cs.tasks))
	}
}

// ctx 取消同样终止链条
func TestRunloopSampler_CtxCancelTerminatesChain(t *testing.T) {
	cs := &captureScheduler{}
	ctx, cancel := context.WithCancel(context.Background())
	rs := &RunloopSampler{name: "xs005b", fn: func() string { return "e" }, scheduleFn: cs.schedule}

	rs.Run(ctx)
	if len(cs.tasks) != 1 {
		t.Fatalf("first tick should schedule exactly one follow-up, got %d", len(cs.tasks))
	}

	cancel()
	cs.tasks[0]()
	if len(cs.tasks) != 1 {
		t.Fatalf("sampler rescheduled after ctx cancel (XS-005): %d tasks", len(cs.tasks))
	}
}
