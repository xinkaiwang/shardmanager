package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/libs/xklib/krunloop"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/etcdprov"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

// syncShardPlan: this must called in runloop
// this will modify AllShards, and call FlushShardState
func (ss *ServiceState) syncShardPlan(ctx context.Context, shardPlan []*smgjson.ShardLineJson) {
	// 注意：此函数已经在 runloop 中调用，它是 ServiceState 的方法
	// ServiceState 已经有限制只能在 runloop 中修改，因此这里不需要额外的锁
	// runloop 是单线程的，因此这里的 map 操作是安全的

	// 如果外部仍需要安全地访问这个 map，ServiceState 应该提供安全的访问方法
	// 此处的问题可能是测试代码直接访问了 ss.AllShards 而不通过安全方法

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
			// add new shard
			shard := NewShardStateByPlan(shardLine)
			ss.AllShards[shard.ShardId] = shard
			inserted = append(inserted, shard.ShardId)
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
			sp.parent.PostEvent(&ShardPlanUpdateEvent{ShardPlan: shardPlan})
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
	ss.syncShardPlan(ctx, spue.ShardPlan)
}
