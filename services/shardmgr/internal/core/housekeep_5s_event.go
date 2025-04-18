package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
)

// Housekeep1sEvent implements krunloop.IEvent[*ServiceState] interface
type Housekeep5sEvent struct {
}

func (te *Housekeep5sEvent) GetName() string {
	return "TimerEvent"
}

func (te *Housekeep5sEvent) Process(ctx context.Context, ss *ServiceState) {
	// ss.checkWorkerTombStone(ctx)
	ss.checkShardTombStone(ctx)
	kcommon.ScheduleRun(5*1000, func() { // 5s
		ss.PostEvent(NewHousekeep5sEvent())
	})
}

func NewHousekeep5sEvent() *Housekeep5sEvent {
	return &Housekeep5sEvent{}
}

func (ss *ServiceState) checkShardTombStone(ctx context.Context) {
	// check shard tombstone
	for _, shard := range ss.AllShards {
		if !shard.LameDuck {
			continue
		}
		if len(shard.Replicas) > 0 {
			continue
		}
		// shard is tombstone, delete it
		delete(ss.AllShards, shard.ShardId)
		ss.storeProvider.StoreShardState(shard.ShardId, nil)
		klogging.Info(ctx).With("shardId", shard.ShardId).Log("checkShardTombStone", "delete shardState")
	}
}
