package krunloop

import (
	"context"
	"sync/atomic"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kmetrics"
)

var (
	RunLoopSamplerMetric = kmetrics.CreateKmetric(context.Background(), "runloop_sample_ct", "periodic samples of the event currently being processed (busy-profile of the runloop; 'none' = idle)", []string{"name", "event"}).CountOnly()
)

type RunloopSampler struct {
	name    string
	fn      SampleFunc
	stopped atomic.Bool // XS-005: 终止自我重调度链

	// scheduleFn 为测试注入缝（nil = kcommon.ScheduleRun）。实例级而非全局
	// mock：并发测试二进制里换全局 TimeProvider 会与残留定时器回调数据竞争。
	scheduleFn func(delayMs int, fn func())
}

func (rs *RunloopSampler) schedule(delayMs int, fn func()) {
	if rs.scheduleFn != nil {
		rs.scheduleFn(delayMs, fn)
		return
	}
	kcommon.ScheduleRun(delayMs, fn)
}

type SampleFunc func() string

// name is used for logging/metrics purposes only
func NewRunloopSampler(ctx context.Context, fn SampleFunc, name string) *RunloopSampler {
	sampler := &RunloopSampler{
		name: name,
		fn:   fn,
	}
	go sampler.Run(ctx)
	return sampler
}

func (rs *RunloopSampler) Run(ctx context.Context) {
	// XS-005: 无终止条件的自我重调度曾使每个 RunLoop 泄漏一条永久定时器链。
	// Stop() 或 ctx 取消都会在下一 tick 终止链条。
	if rs.stopped.Load() || ctx.Err() != nil {
		return
	}
	current := rs.fn()
	if current == "" {
		current = "none"
	}
	RunLoopSamplerMetric.GetTimeSequence(ctx, rs.name, current).Add(1)
	// 采样周期 20ms：每 tick 成本为一次原子读 + 一次计数加，量级无害；
	// 20 这个值是取用方便而非实测定值（见 research/2026_0809.XklibSmellScan）。
	rs.schedule(20, func() {
		rs.Run(ctx)
	})
}

// Stop terminates the sampler's reschedule chain (idempotent; takes effect
// at the next tick, ≤1 sampling period away).
func (rs *RunloopSampler) Stop() {
	rs.stopped.Store(true)
}

func (rs *RunloopSampler) InitTimeSeries(ctx context.Context, names ...string) {
	for _, name := range names {
		RunLoopSamplerMetric.GetTimeSequence(ctx, rs.name, name).Touch()
	}
}
