package main

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kmetrics"
)

// logMetric: 单指标 + drop tag（2026-08-09 KLOG-002 决策，见
// research/2026-08-09-ctx-info-revisit/notes.md）：
//   - log_size_count{level,event,drop}: 日志行数；总尝试量 = sum(全部)，
//     压掉量 = sum(drop="1")
//   - log_size_sum{level,event,drop}:   日志字节数（drop="1" 行恒为 0——
//     被压掉的日志没有 record，字节数不可知，结构性约束如实呈现）
//
// drop="1" 的行 event 恒为空串（Enabled 阶段拿不到 event，level 粒度）。
var logMetric = kmetrics.CreateKmetric(context.Background(), "log_size",
	"log lines (count) and bytes (sum) by level/event/drop", []string{"level", "event", "drop"})

// KloggingMetricsReporter implements klogging.MetricsReporter（KLOG-001 接线）。
type KloggingMetricsReporter struct{}

func NewKloggingMetricsReporter() *KloggingMetricsReporter {
	return &KloggingMetricsReporter{}
}

func (r *KloggingMetricsReporter) ReportLog(ctx context.Context, level, event string, size int, dropped bool) {
	drop := "0"
	if dropped {
		drop = "1"
	}
	logMetric.GetTimeSequence(ctx, level, event, drop).Add(int64(size))
}
