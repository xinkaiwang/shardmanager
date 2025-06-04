package main

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kmetrics"
)

var (
	LogSizeBytesMetrics  = kmetrics.CreateKmetric(context.Background(), "klogging_volume_byte", "log size in byte (don't include skipped events)", []string{"level", "event"})
	LogErrorCountMetrics = kmetrics.CreateKmetric(context.Background(), "klogging_breif_count", "log event count (include those skipped)", []string{"level", "event", "logged"}).CountOnly()
)

// MyLoggerMetrcsReporter implements klogging.MyLoggerMetrcsReporter interface
type MyLoggerMetrcsReporter struct {
}

func NewMyLoggerMetrcsReporter() *MyLoggerMetrcsReporter {
	return &MyLoggerMetrcsReporter{}
}

func (lmr *MyLoggerMetrcsReporter) ReportLogSizeBytes(ctx context.Context, size int, logLevel, eventType string) {
	// Implement logic to report log size in bytes
	LogSizeBytesMetrics.GetTimeSequence(ctx, logLevel, eventType).Add(int64(size))
}

func (lmr *MyLoggerMetrcsReporter) ReportLogErrorCount(ctx context.Context, count int, logLevel, eventType string, isLogged bool) {
	// Implement logic to report log error count
	isLoggedStr := "false"
	if isLogged {
		isLoggedStr = "true"
	}
	LogErrorCountMetrics.GetTimeSequence(ctx, logLevel, eventType, isLoggedStr).Add(int64(count))
}
