package main

import (
	"context"
	"fmt"

	"github.com/xinkaiwang/shardmanager/libs/cougar/cougar"
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

func (shard *MyShard) GetShardQpm(ctx context.Context) int64 {
	return 1000
}

func main() {
	// 显示版本信息
	ctx := context.Background()
	fmt.Printf("Cougar Demo %s (Commit: %s, Built: %s)\n", Version, GitCommit, BuildTime)
	// 创建 CougarBuilder
	builder := cougar.NewCougarBuilder()
	cougarApp := NewMyCougarApp()
	builder.WithCougarApp(cougarApp)
	// builder.WithNotifyShardChange(func(shardId data.ShardId, action cougar.CougarAction) {
	// 	fmt.Printf("Shard %s changed\n", shardId)
	// 	kcommon.ScheduleRun(1000, func() {
	// 		if action == cougar.CA_AddShard {
	// 			// fmt.Printf("Adding shard %s\n", shardId)
	// 			cougarInstance.VisitState(func(state *cougar.CougarState) string {
	// 				shard := state.AllShards[shardId]
	// 				shard.CurrentConfirmedState = cougarjson.CAS_Ready
	// 				fmt.Printf("Added shard %s\n", shardId)
	// 				return "AddedSucc"
	// 			})
	// 		} else if action == cougar.CA_RemoveShard {
	// 			// fmt.Printf("Removing shard %s\n", shardId)
	// 			cougarInstance.VisitState(func(state *cougar.CougarState) string {
	// 				shard, ok := state.AllShards[shardId]
	// 				if !ok {
	// 					// 如果 shard 不存在，则直接返回
	// 					klogging.Warning(ctx).With("shardId", shardId).Log("shard not found", "")
	// 					return "ShardNotFound"
	// 				}
	// 				shard.CurrentConfirmedState = cougarjson.CAS_Dropped
	// 				fmt.Printf("Removed shard %s\n", shardId)
	// 				return "RemovedSucc"
	// 			})
	// 		} else if action == cougar.CA_UpdateShard {
	// 			// fmt.Printf("Updating shard %s\n", shardId)
	// 			cougarInstance.VisitState(func(state *cougar.CougarState) string {
	// 				fmt.Printf("Updated shard %s\n", shardId)
	// 				return "UpdatedSucc"
	// 			})
	// 		}
	// 	})
	// })
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
