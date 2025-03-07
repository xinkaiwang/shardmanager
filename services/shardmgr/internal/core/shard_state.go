package core

import (
	"context"

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
	if shard.Hints == newHints {
		return false
	}
	shard.Hints = newHints
	return true
}

func (shard *ShardState) MarkAsSoftDelete(ctx context.Context) {
	shard.LameDuck = true
	for _, replica := range shard.Replicas {
		replica.MarkAsSoftDelete(ctx)
	}
}

func (ss *ServiceState) FlushShardState(ctx context.Context, updated []data.ShardId, inserted []data.ShardId, deleted []data.ShardId) {
	for _, shardId := range updated {
		ss.StoreProvider.StoreShardState(shardId, ss.AllShards[shardId].ToJson())
	}
	for _, shardId := range inserted {
		ss.StoreProvider.StoreShardState(shardId, ss.AllShards[shardId].ToJson())
	}
	for _, shardId := range deleted {
		ss.StoreProvider.StoreShardState(shardId, nil)
	}
}

func (shard *ShardState) ToJson() *smgjson.ShardStateJson {
	obj := smgjson.NewShardStateJson(shard.ShardId)
	for _, replica := range shard.Replicas {
		obj.Resplicas[replica.ReplicaIdx] = replica.ToJson()
	}
	return obj
}
