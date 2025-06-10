package krunloop

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kmetrics"
)

var (
	RunLoopSamplerMetric = kmetrics.CreateKmetric(context.Background(), "runloop_sample_ct", "desc", []string{"name", "event"}).CountOnly()
)

type RunloopSampler struct {
	name string
	fn   SampleFunc
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
	current := rs.fn()
	if current == "" {
		current = "none"
	}
	RunLoopSamplerMetric.GetTimeSequence(ctx, rs.name, current).Add(1)
	kcommon.ScheduleRun(20, func() { // sleep 20ms = 50Hz
		rs.Run(ctx)
	})
}

func (rs *RunloopSampler) InitTimeSeries(ctx context.Context, names ...string) {
	for _, name := range names {
		RunLoopSamplerMetric.GetTimeSequence(ctx, rs.name, name).Touch()
	}
}
