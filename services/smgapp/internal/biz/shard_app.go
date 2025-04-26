package biz

import (
	"context"
	"sync"

	"github.com/xinkaiwang/shardmanager/libs/cougar/cougar"
	"github.com/xinkaiwang/shardmanager/libs/unicorn/data"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
)

/********************************** MyCougarApp **********************************/

// MyCougarApp implements cougarapp.CougarApp interface
type MyCougarApp struct {
	Mu     sync.Mutex
	Shards map[data.ShardId]*MyAppShard
}

func NewMyCougarApp() *MyCougarApp {
	return &MyCougarApp{
		Shards: make(map[data.ShardId]*MyAppShard),
	}
}

// AddShard implements cougarapp.CougarApp interface
func (app *MyCougarApp) AddShard(ctx context.Context, shardInfo *cougar.ShardInfo, chDone chan struct{}) cougar.AppShard {
	klogging.Info(ctx).With("shardId", shardInfo.ShardId).Log("MyCougarApp", "AddShard")
	shard := &MyAppShard{
		shardInfo: shardInfo,
	}
	app.Mu.Lock()
	defer app.Mu.Unlock()
	app.Shards[shardInfo.ShardId] = shard
	close(chDone)
	return shard
}

func (app *MyCougarApp) DropShard(ctx context.Context, shardId data.ShardId, chDone chan struct{}) {
	klogging.Info(ctx).With("shardId", shardId).Log("MyCougarApp", "DropShard")
	app.Mu.Lock()
	defer app.Mu.Unlock()
	delete(app.Shards, shardId)
	close(chDone)
}

func (app *MyCougarApp) GetShard(ctx context.Context, shardId data.ShardId) *MyAppShard {
	if shardId == "" {
		ke := kerror.Create("MyCougarApp", "shardIdEmpty").WithErrorCode(kerror.EC_INVALID_PARAMETER)
		panic(ke)
	}
	app.Mu.Lock()
	defer app.Mu.Unlock()
	shard, ok := app.Shards[shardId]
	if !ok {
		ke := kerror.Create("MyCougarApp", "ShardId not found").With("shardId", shardId).WithErrorCode(kerror.EC_INTERNAL_ERROR)
		panic(ke)
	}
	return shard
}

/********************************** MyAppShard **********************************/

// MyAppShard implements cougarapp.AppShard interface
type MyAppShard struct {
	shardInfo  *cougar.ShardInfo
	currentQpm int64
}

func NewAppShard(shardInfo *cougar.ShardInfo) *MyAppShard {
	return &MyAppShard{
		shardInfo:  shardInfo,
		currentQpm: 300,
	}
}

func (shard *MyAppShard) UpdateShard(ctx context.Context, shardInfo *cougar.ShardInfo) {
	klogging.Info(ctx).With("shardId", shardInfo.ShardId).Log("MyAppShard", "UpdateShard")
	shard.shardInfo = shardInfo
}

func (shard *MyAppShard) GetShardQpm(ctx context.Context) int64 {
	max := 600
	min := 0
	shard.currentQpm += int64(kcommon.RandomInt(ctx, 11) - 5)
	if shard.currentQpm > int64(max) {
		shard.currentQpm = int64(max)
	} else if shard.currentQpm < int64(min) {
		shard.currentQpm = int64(min)
	}
	klogging.Info(ctx).With("shardId", shard.shardInfo.ShardId).With("currentQpm", shard.currentQpm).Log("MyAppShard", "GetShardQpm")
	return shard.currentQpm // TODO: implement this
}

func (shard *MyAppShard) Ping(ctx context.Context, name string) string {
	return "pong, name=" + name + ", shardId=" + string(shard.shardInfo.ShardId)
}
