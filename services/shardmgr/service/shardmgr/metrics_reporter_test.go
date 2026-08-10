package main

import (
	"context"
	"testing"
)

// KLOG-001/002 验收：emitted 与 dropped 进入同一指标的不同 drop tag 序列，
// count 记行数、sum 记字节数（dropped 行 size=0）。
func TestKloggingMetricsReporter(t *testing.T) {
	ctx := context.Background()
	r := NewKloggingMetricsReporter()

	r.ReportLog(ctx, "INFO", "TestEvent", 100, false)
	r.ReportLog(ctx, "INFO", "TestEvent", 50, false)
	r.ReportLog(ctx, "DEBUG", "", 0, true)

	count, sum := logMetric.GetTimeSequence(ctx, "INFO", "TestEvent", "0").Get()
	if count != 2 || sum != 150 {
		t.Errorf("emitted series: count=%d sum=%d, want 2/150", count, sum)
	}

	count, sum = logMetric.GetTimeSequence(ctx, "DEBUG", "", "1").Get()
	if count != 1 || sum != 0 {
		t.Errorf("dropped series: count=%d sum=%d, want 1/0", count, sum)
	}
}
