package main

import (
	"context"
	"fmt"

	"github.com/xinkaiwang/shardmanager/libs/cougar/cougar"
	"github.com/xinkaiwang/shardmanager/libs/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/libs/unicorn/data"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
)

// 版本信息，通过 ldflags 在构建时注入
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

// MyCougarApp implements the cougar.CougarApp interface
type MyCougarApp struct {
}

func NewMyCougarApp() *MyCougarApp {
	return &MyCougarApp{}
}

func (app *MyCougarApp) AddShard(ctx context.Context, shard *cougar.ShardInfo, chDone chan struct{}) cougar.AppShard {
	return &MyShard{}
}

func (app *MyCougarApp) DropShard(ctx context.Context, shardId data.ShardId, chDone chan struct{}) {
	close(chDone)
}

// MyShard implements cougar.AppShard
type MyShard struct {
}

func (shard *MyShard) UpdateShard(ctx context.Context, shardInfo *cougar.ShardInfo) {
	// Update shard information if needed
}

func (shard *MyShard) GetShardStats(ctx context.Context) cougarjson.ShardStats {
	return cougarjson.ShardStats{
		Qpm:   100,
		MemMb: 1024,
	}
}

func main() {
	// 显示版本信息
	ctx := context.Background()
	fmt.Printf("Cougar Demo %s (Commit: %s, Built: %s)\n", Version, GitCommit, BuildTime)
	// 创建 CougarBuilder
	builder := cougar.NewCougarBuilder()
	cougarApp := NewMyCougarApp()
	builder.WithCougarApp(cougarApp)
	workerInfo := cougar.NewWorkerInfo(
		"worker1",
		"session1",
		"localhost:8080",
		kcommon.GetWallTimeMs(),
		100,
		16*1024,
		map[string]string{},
		data.ST_MEMORY,
	)
	builder.WithWorkerInfo(workerInfo)
	// 创建 Cougar
	builder.Build(ctx)

	fmt.Println("Cougar created, waiting for state changes...")
	fmt.Scanln()

	fmt.Println("Hello, World!")
}
