package krunloop

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kmetrics"
)

var (
	RunLoopSamplerMetric = kmetrics.CreateKmetric(context.Background(), "runloop_sample_ct", "desc", []string{"event"}).CountOnly()
)

type RunloopSampler struct {
	fn SampleFunc
}

type SampleFunc func() string

func NewRunloopSampler(ctx context.Context, fn SampleFunc) *RunloopSampler {
	sampler := &RunloopSampler{
		fn: fn,
	}
	go sampler.Run(ctx)
	return sampler
}

func (s *RunloopSampler) Run(ctx context.Context) {
	current := s.fn()
	if current == "" {
		current = "none"
	}
	RunLoopSamplerMetric.GetTimeSequence(ctx, current).Add(1)
	kcommon.ScheduleRun(20, func() { // sleep 20ms = 50Hz
		s.Run(ctx)
	})
}
