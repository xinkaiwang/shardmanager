package cougarapp

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/cougar/cougar"
)

type CougarApp interface {
	AddShard(ctx context.Context, shard cougar.ShardInfo) AppShard
	DropShard(ctx context.Context, shardId string, chDone chan struct{})
}

type AppShard interface {
	IsReady(ctx context.Context) bool
	GetShardQpm(ctx context.Context) int64
}
