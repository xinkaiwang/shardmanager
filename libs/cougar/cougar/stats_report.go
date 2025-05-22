package cougar

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
)

func (c *CougarImpl) StatsReport(ctx context.Context) {
	c.statsReportInternal(ctx)
	kcommon.ScheduleRun(30*1000, func() {
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
