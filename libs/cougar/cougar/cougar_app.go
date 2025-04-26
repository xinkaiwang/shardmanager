package cougar

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/unicorn/data"
)

type CougarApp interface {
	AddShard(ctx context.Context, shard *ShardInfo, chDone chan struct{}) AppShard // must have
	DropShard(ctx context.Context, shardId data.ShardId, chDone chan struct{})     // must have
}

type AppShard interface {
	UpdateShard(ctx context.Context, shard *ShardInfo) // optional: this is no-op for most applications
	GetShardQpm(ctx context.Context) int64
}
