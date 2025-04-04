package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
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

func NewShardStateByPlan(shardLine *smgjson.ShardLineJson, defCfg config.ShardConfig) *ShardState {
	// cfg
	if shardLine.Hints != nil {
		if shardLine.Hints.MinReplicaCount != nil {
			defCfg.MinReplicaCount = int(*shardLine.Hints.MinReplicaCount)
		}
		if shardLine.Hints.MaxReplicaCount != nil {
			defCfg.MaxReplicaCount = int(*shardLine.Hints.MaxReplicaCount)
		}
		if shardLine.Hints.MoveType != nil {
			defCfg.MovePolicy = *shardLine.Hints.MoveType
		}
	}
	return &ShardState{
		ShardId:          data.ShardId(shardLine.ShardName),
		Replicas:         make(map[data.ReplicaIdx]*ReplicaState),
		Hints:            defCfg,
		CustomProperties: shardLine.CustomProperties,
		LameDuck:         false,
	}
}

// return 1 if shard state is updated, 0 if not
func (ss *ServiceState) UpdateShardStateByPlan(shard *ShardState, shardLine *smgjson.ShardLineJson) bool {
	dirty := false
	newHints := ss.ServiceConfig.ShardConfig
	if shardLine.Hints != nil {
		if shardLine.Hints.MinReplicaCount != nil {
			newHints.MinReplicaCount = int(*shardLine.Hints.MinReplicaCount)
		}
		if shardLine.Hints.MaxReplicaCount != nil {
			newHints.MaxReplicaCount = int(*shardLine.Hints.MaxReplicaCount)
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
		klogging.Info(ctx).With("shardId", shardId).With("time", kcommon.GetWallTimeMs()).Log("FlushShardState", "更新分片")
		ss.storeProvider.StoreShardState(shardId, ss.AllShards[shardId].ToJson())
	}
	for _, shardId := range inserted {
		klogging.Info(ctx).With("shardId", shardId).With("time", kcommon.GetWallTimeMs()).Log("FlushShardState", "插入分片")
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

// ReEvaluateReplicaCount: based on the current replica count and the minimum replica count, add new replicas if needed
func (shard *ShardState) ReEvaluateReplicaCount() int {
	// 重新计算副本数
	replicaCount := 0
	largestReplicaIdx := data.ReplicaIdx(0)
	for _, replica := range shard.Replicas {
		if !replica.LameDuck {
			replicaCount++
		}
		if replica.ReplicaIdx > largestReplicaIdx {
			largestReplicaIdx = replica.ReplicaIdx
		}
	}
	// add new replicas if needed
	for replicaCount < shard.Hints.MinReplicaCount {
		// add new replica
		// step 1: find the next available replica index
		nextReplicaIdx := shard.findNextAvailableReplicaIndex()
		// step 2: create a new replica
		replica := NewReplicaState(shard.ShardId, nextReplicaIdx)
		shard.Replicas[nextReplicaIdx] = replica
		replicaCount++
	}
	// (soft delete) remove extra replicas if needed
	for replicaCount > shard.Hints.MaxReplicaCount {
		// remove the last replica
		if replica, ok := shard.Replicas[largestReplicaIdx]; ok {
			if !replica.LameDuck {
				replica.LameDuck = true
				replicaCount--
			}
		}
		largestReplicaIdx--
	}
	return replicaCount
}

func (shard *ShardState) findNextAvailableReplicaIndex() data.ReplicaIdx {
	// find the next available replica index
	i := 0
	if _, ok := shard.Replicas[data.ReplicaIdx(i)]; ok {
		i++
	}
	return data.ReplicaIdx(i)
}
