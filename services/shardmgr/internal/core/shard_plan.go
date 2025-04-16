package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/libs/xklib/krunloop"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/etcdprov"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

// digestStagingShardPlan: this must called in runloop
// this will modify AllShards, and call FlushShardState
func (ss *ServiceState) digestStagingShardPlan(ctx context.Context) {
	shardPlan := ss.stagingShardPlan

	// compare with current shard state
	needRemove := map[data.ShardId]*ShardState{}
	for _, shard := range ss.AllShards {
		needRemove[shard.ShardId] = shard
	}
	updated := []data.ShardId{}
	inserted := []data.ShardId{}
	deleted := []data.ShardId{}
	// Based on what we have in shardPlan, we will update shard state, add new shard if not exist, and remove shard if not in shardPlan, update shard state if shardPlan is different
	for _, shardLine := range shardPlan {
		shardId := data.ShardId(shardLine.ShardName)
		if shard, ok := ss.AllShards[shardId]; ok {
			// update shard state
			if ss.UpdateShardStateByPlan(shard, shardLine) {
				updated = append(updated, shard.ShardId)
			}
			delete(needRemove, shard.ShardId)
		} else {
			// add new shardState
			shardState := NewShardStateByPlan(shardLine, ss.ServiceConfig.ShardConfig)
			shardState.ReEvaluateReplicaCount()
			ss.AllShards[shardState.ShardId] = shardState
			inserted = append(inserted, shardState.ShardId)
		}
	}
	// remove shard if not in shardPlan
	for _, shard := range needRemove {
		deleted = append(deleted, shard.ShardId)
		// soft delete
		shard.MarkAsSoftDelete(ctx)
	}
	// log
	klogging.Info(ctx).With("updated", updated).With("inserted", inserted).With("deleted", deleted).Log("syncShardPlan", "done")
	ss.FlushShardState(ctx, updated, inserted, deleted)
	ss.reCreateSnapshotBatchManager.TrySchedule(ctx)
}

type ShardPlanWatcher struct {
	parent krunloop.EventPoster[*ServiceState]
	ch     chan etcdprov.EtcdKvItem
}

func NewShardPlanWatcher(ctx context.Context, parent *ServiceState, currentShardPlanRevision etcdprov.EtcdRevision) *ShardPlanWatcher {
	sp := &ShardPlanWatcher{
		parent: parent,
		ch:     etcdprov.GetCurrentEtcdProvider(ctx).WatchByPrefix(ctx, parent.PathManager.GetShardPlanPath(), currentShardPlanRevision),
	}
	go sp.run(ctx)
	return sp
}

func (sp *ShardPlanWatcher) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case item, ok := <-sp.ch:
			if !ok {
				return
			}
			shardPlan := smgjson.ParseShardPlan(item.Value)
			krunloop.VisitResource(sp.parent, func(ss *ServiceState) {
				ss.stagingShardPlan = shardPlan
				ss.syncShardsBatchManager.TrySchedule(ctx)
			})
		}
	}
}

// implements IEvent[*ServiceState]
type ShardPlanUpdateEvent struct {
	ShardPlan []*smgjson.ShardLineJson
}

func (spue *ShardPlanUpdateEvent) GetName() string {
	return "ShardPlanUpdateEvent"
}

func (spue *ShardPlanUpdateEvent) Process(ctx context.Context, ss *ServiceState) {
	ss.digestStagingShardPlan(ctx)
}
