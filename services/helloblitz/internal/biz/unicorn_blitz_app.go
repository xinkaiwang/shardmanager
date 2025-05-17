package biz

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/unicorn/data"
	"github.com/xinkaiwang/shardmanager/libs/unicorn/unicorn"
	"github.com/xinkaiwang/shardmanager/libs/unicorn/unicornjson"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
)

type UnicornBlitzApp struct {
	uc *unicorn.Unicorn
}

func NewUnicornBlitzApp(ctx context.Context) *UnicornBlitzApp {
	app := &UnicornBlitzApp{}
	builder := unicorn.NewUnicornBuilder()
	builder.WithRoutingTreeBuilder(func(workers map[data.WorkerId]*unicornjson.WorkerEntryJson) unicorn.RoutingTree {
		return unicorn.RangeBasedShardIdTreeBuilder(workers)
	})
	app.uc = builder.Build(ctx)
	return app
}

func (app *UnicornBlitzApp) GetTargetByShardingKey(ctx context.Context, shardingKey uint32) *unicorn.RoutingTarget {
	return app.uc.GetCurrentTree().FindShardByShardingKey(data.ShardingKey(shardingKey))
}

func (app *UnicornBlitzApp) GetTargetByObjectKey(ctx context.Context, objectId string) *unicorn.RoutingTarget {
	key := unicorn.JavaStringHashCode(objectId)
	return app.uc.GetCurrentTree().FindShardByShardingKey(data.ShardingKey(key))
}

func (app *UnicornBlitzApp) RunLoadTest(ctx context.Context, shardingKey uint32) {
	target := app.GetTargetByShardingKey(ctx, shardingKey)
	if target == nil {
		panic("target is nil")
	}
	klogging.Info(ctx).With("shardingKey", shardingKey).With("target", target).Log("UnicornBlitzApp", "RunLoadTest")
}
