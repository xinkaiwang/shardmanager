package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

type ShardState struct {
	ShardId          data.ShardId
	Replicas         map[data.ReplicaIdx]*ReplicaState
	Hints            config.ShardConfig
	CustomProperties map[string]string
	LameDuck         bool
}

func NewShardState(shardId data.ShardId) *ShardState {
	return &ShardState{ShardId: shardId}
}

func NewShardStateByPlan(shardLine *smgjson.ShardLineJson) *ShardState {
	// TODO
	return &ShardState{
		ShardId: data.ShardId(shardLine.ShardName),
	}
}

// return 1 if shard state is updated, 0 if not
func (ss *ServiceState) UpdateShardStateByPlan(shard *ShardState, shardLine *smgjson.ShardLineJson) bool {
	dirty := false
	newHints := ss.ServiceConfig.ShardConfig
	if shardLine.Hints != nil {
		if shardLine.Hints.MinReplicaCount != nil {
			newHints.MinReplicaCount = *shardLine.Hints.MinReplicaCount
		}
		if shardLine.Hints.MaxReplicaCount != nil {
			newHints.MaxReplicaCount = *shardLine.Hints.MaxReplicaCount
		}
		if shardLine.Hints.MoveType != nil {
			newHints.MovePolicy = *shardLine.Hints.MoveType
		}
	}
	if shard.Hints != newHints {
		shard.Hints = newHints
		dirty = true
	}
	if shard.LameDuck { // 如果分片已经被标记为软删除，则undo软删除, 重新激活分片
		shard.LameDuck = false
		dirty = true
	}
	return dirty
}

func (shard *ShardState) MarkAsSoftDelete(ctx context.Context) {
	shard.LameDuck = true
	for _, replica := range shard.Replicas {
		replica.MarkAsSoftDelete(ctx)
	}
}

func (ss *ServiceState) FlushShardState(ctx context.Context, updated []data.ShardId, inserted []data.ShardId, deleted []data.ShardId) {
	klogging.Info(ctx).With("updated", updated).With("inserted", inserted).With("deleted", deleted).Log("FlushShardState", "开始刷新分片状态")

	for _, shardId := range updated {
		klogging.Info(ctx).With("shardId", shardId).Log("FlushShardState", "更新分片")
		ss.storeProvider.StoreShardState(shardId, ss.AllShards[shardId].ToJson())
	}
	for _, shardId := range inserted {
		klogging.Info(ctx).With("shardId", shardId).Log("FlushShardState", "插入分片")
		ss.storeProvider.StoreShardState(shardId, ss.AllShards[shardId].ToJson())
	}
	for _, shardId := range deleted {
		if shard, ok := ss.AllShards[shardId]; ok {
			// 使用当前分片的状态创建软删除记录
			// 分片已经在MarkAsSoftDelete中设置了LameDuck=true
			ss.storeProvider.StoreShardState(shardId, shard.ToJson())
		} else {
			// 如果分片已经不在内存中，则删除分片状态
			ss.storeProvider.StoreShardState(shardId, nil)
		}
	}
}

func (shard *ShardState) ToJson() *smgjson.ShardStateJson {
	obj := smgjson.NewShardStateJson(shard.ShardId)
	for _, replica := range shard.Replicas {
		obj.Resplicas[replica.ReplicaIdx] = replica.ToJson()
	}
	obj.LameDuck = smgjson.Bool2Int8(shard.LameDuck)
	return obj
}
