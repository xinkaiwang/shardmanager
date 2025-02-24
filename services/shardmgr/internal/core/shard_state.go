package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

type ShardState struct {
	ShardId data.ShardId
}

func NewShardState(shardId data.ShardId) *ShardState {
	return &ShardState{ShardId: shardId}
}

func NewShardStateByPlan(shardLine *smgjson.ShardLine) *ShardState {
	// TODO
	return &ShardState{
		ShardId: data.ShardId(shardLine.ShardName),
	}
}

// return 1 if shard state is updated, 0 if not
func (ss *ShardState) UpdateShardStateByPlan(shardLine *smgjson.ShardLine) bool {
	// TODO
	return false
}

func (ss *ShardState) MarkAsSoftDelete(ctx context.Context) {
	// TODO
}

func (ss *ServiceState) FlushShardState(ctx context.Context, updated []data.ShardId, inserted []data.ShardId, deleted []data.ShardId) {
	// TODO
}
