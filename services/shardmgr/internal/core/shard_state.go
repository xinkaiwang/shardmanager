package core

import "github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"

type ShardState struct {
	ShardId data.ShardId
}

func NewShardState(shardId data.ShardId) *ShardState {
	return &ShardState{ShardId: shardId}
}
