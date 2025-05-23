package cougar

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
)

func GetStatsReportIntervalMs() int {
	intervalSec := kcommon.GetEnvInt("COUGAR_STATS_REPORT_INTERVAL_S", 30)
	return intervalSec * 1000
}

func (c *CougarImpl) StatsReport(ctx context.Context) {
	c.statsReportInternal(ctx)
	intervalMs := GetStatsReportIntervalMs()
	kcommon.ScheduleRun(intervalMs, func() {
		c.StatsReport(ctx)
	})
}

func (c *CougarImpl) statsReportInternal(ctx context.Context) {
	// step 1: collect stats
	c.VisitStateAndWait(func(state *CougarState) string {
		for _, shardInfo := range c.cougarState.AllShards {
			stats := shardInfo.AppShard.GetShardStats(ctx)
			shardInfo.stats = func() *cougarjson.ShardStats {
				return &stats
			}()
		}
		return "stats_report"
	})
}
