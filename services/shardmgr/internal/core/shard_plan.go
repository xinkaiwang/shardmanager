package core

import (
	"context"
	"log/slog"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kmetrics"
	"github.com/xinkaiwang/shardmanager/libs/xklib/krunloop"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/etcdprov"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

var (
	shardPlanUpdateMetrics = kmetrics.CreateKmetric(context.Background(), "shard_plan_update_bytes", "shard plan update", []string{})
)

// digestStagingShardPlan: this must called in runloop
// this will modify AllShards, and call FlushShardState
func (ss *ServiceState) digestStagingShardPlan(ctx context.Context) bool {
	shardPlan := ss.stagingShardPlan
	// var passiveMoves []costfunc.PassiveMove

	// compare with current shard state
	needRemove := map[data.ShardId]*ShardState{}
	for _, shard := range ss.AllShards {
		needRemove[shard.ShardId] = shard
	}
	updated := []data.ShardId{}
	inserted := []data.ShardId{}
	deleted := []data.ShardId{}
	unchanged := 0
	// Based on what we have in shardPlan, we will update shard state, 1) add new shard if not exist, and 2) remove shard if not in shardPlan, 3) update shard state if shardPlan is different
	for _, shardLine := range shardPlan {
		shardId := data.ShardId(shardLine.ShardName)
		if shard, ok := ss.AllShards[shardId]; ok {
			// 3) update
			dirtyFlags := ss.UpdateShardStateByPlan(shard, shardLine)
			if dirtyFlags.IsDirty() {
				updated = append(updated, shard.ShardId)
				shard.LastUpdateTimeMs = kcommon.GetWallTimeMs()
				shard.LastUpdateReason = dirtyFlags.String()
			} else {
				unchanged++
			}
			delete(needRemove, shard.ShardId)
		} else {
			// 1) add new
			shardState := NewShardStateByPlan(shardLine, ss.ServiceConfig.ShardConfig)
			shardState.reEvaluateReplicaCount()
			ss.AllShards[shardState.ShardId] = shardState
			inserted = append(inserted, shardState.ShardId)
		}
	}
	// 2) remove
	for _, shard := range needRemove {
		if !shard.LameDuck {
			deleted = append(deleted, shard.ShardId)
			// soft delete
			shard.MarkAsSoftDelete(ctx)
			shard.LastUpdateTimeMs = kcommon.GetWallTimeMs()
			shard.LastUpdateReason = "lameDuck"
		}
	}
	// log
	dirty := len(updated) + len(inserted) + len(deleted)
	slog.InfoContext(ctx, "done",
		slog.String("event", "syncShardPlan"),
		slog.Any("updated", updated),
		slog.Any("inserted", inserted),
		slog.Any("deleted", deleted),
		slog.Any("unchanged", unchanged),
		slog.Any("dirty", dirty))
	if dirty != 0 {
		ss.FlushShardState(ctx, updated, inserted, deleted)
		// ss.ReCreateSnapshot(ctx, "digestStagingShardPlan")
		// ss.reCreateSnapshotBatchManager.TrySchedule(ctx, "digestStagingShardPlan")
	}
	return dirty != 0
}

type ShardPlanWatcher struct {
	parent  krunloop.EventPoster[*ServiceState]
	ch      chan etcdprov.EtcdKvItem
	path    string
	cancel  context.CancelFunc // 用于stop watcher
	stop    chan struct{}
	stopped chan struct{}
}

func NewShardPlanWatcher(ctx context.Context, parent *ServiceState, currentShardPlanRevision etcdprov.EtcdRevision) *ShardPlanWatcher {
	path := parent.PathManager.GetShardPlanPath()
	ctx, cancel := context.WithCancel(ctx)
	sp := &ShardPlanWatcher{
		parent:  parent,
		ch:      etcdprov.GetCurrentEtcdProvider(ctx).WatchByPrefix(ctx, path, currentShardPlanRevision),
		path:    path,
		cancel:  cancel,
		stop:    make(chan struct{}),
		stopped: make(chan struct{}),
	}
	sp.InitMetrics(ctx) // 初始化指标
	go sp.run(ctx)      // spw = ShardPlanWatcher
	return sp
}

func (sp *ShardPlanWatcher) run(ctx context.Context) {
	slog.InfoContext(ctx, "start watching",
		slog.String("event", "ShardPlanWatcher"),
		slog.Any("path", sp.path))
	stop := false
	for !stop {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "CtxDone.Exit",
				slog.String("event", "ShardPlanWatcher"))
			stop = true
		case item, ok := <-sp.ch:
			if !ok {
				slog.InfoContext(ctx, "ch closed",
					slog.String("event", "ShardPlanWatcher"))
				return
			}
			slog.InfoContext(ctx, "watcher event",
				slog.String("event", "ShardPlanWatcher"),
				slog.Any("path", item.Key),
				slog.Any("len", len(item.Value)),
				slog.Any("revision", item.ModRevision))
			shardPlanUpdateMetrics.GetTimeSequence(ctx).Add(int64(len(item.Value)))
			shardPlan := smgjson.ParseShardPlan(item.Value)
			krunloop.VisitResource(sp.parent, func(ss *ServiceState) {
				ss.stagingShardPlan = shardPlan
				ss.syncShardsBatchManager.TrySchedule(ctx, "ShardPlanWatcher")
			})
		case <-sp.stop:
			slog.InfoContext(ctx, "stop signal received",
				slog.String("event", "ShardPlanWatcher"))
			stop = true
		}
	}
	close(sp.stopped) // 发送 thread exit 信号
}

func (sp *ShardPlanWatcher) Stop() {
	if sp.cancel != nil {
		sp.cancel()     // 取消上下文，停止 watcher
		sp.cancel = nil // 防止重复调用
	}
	close(sp.stop) // 发送停止信号
}

func (sp *ShardPlanWatcher) StopAndWaitForExit() {
	sp.Stop()
	<-sp.stopped // 等待 run thread exit
	slog.InfoContext(context.Background(), "stopped",
		slog.String("event", "ShardPlanWatcher"))
}

func (sp *ShardPlanWatcher) InitMetrics(ctx context.Context) {
	// 初始化指标
	shardPlanUpdateMetrics.GetTimeSequence(ctx).Touch()
}

// // implements IEvent[*ServiceState]
// type ShardPlanUpdateEvent struct {
// 	ShardPlan []*smgjson.ShardLineJson
// }

// func (spue *ShardPlanUpdateEvent) GetName() string {
// 	return "ShardPlanUpdateEvent"
// }

// func (spue *ShardPlanUpdateEvent) Process(ctx context.Context, ss *ServiceState) {
// 	ss.digestStagingShardPlan(ctx)
// }

func (sp *ShardPlanWatcher) GetCurrentShardPlan(ctx context.Context) string {
	// read from etcd
	shardPlanItem := etcdprov.GetCurrentEtcdProvider(ctx).Get(ctx, sp.path)
	shardPlan := shardPlanItem.Value
	slog.InfoContext(ctx, "GetCurrentShardPlan",
		slog.String("event", "ShardPlanWatcher"),
		slog.Any("path", sp.path),
		slog.Any("len", len(shardPlan)),
		slog.Any("shardPlan", common.StrSizeLimit(shardPlan, 1000)))
	return shardPlan
}

func (sp *ShardPlanWatcher) WriteShardPlan(ctx context.Context, shardPlan string) {
	// write to etcd
	etcdprov.GetCurrentEtcdProvider(ctx).Set(ctx, sp.path, shardPlan)
	slog.InfoContext(ctx, "WriteShardPlan",
		slog.String("event", "ShardPlanWatcher"),
		slog.Any("path", sp.path),
		slog.Any("len", len(shardPlan)),
		slog.Any("shardPlan", common.StrSizeLimit(shardPlan, 1000)))
}
