package main

import (
	"context"
	"fmt"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/services/cougar/cougar"
	"github.com/xinkaiwang/shardmanager/services/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/services/unicorn/data"
)

// 版本信息，通过 ldflags 在构建时注入
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

func main() {
	// 显示版本信息
	ctx := context.Background()
	fmt.Printf("Cougar Demo %s (Commit: %s, Built: %s)\n", Version, GitCommit, BuildTime)
	// 创建 CougarBuilder
	var cougarInstance cougar.Cougar
	builder := cougar.NewCougarBuilder()
	builder.WithNotifyShardChange(func(shardId data.ShardId, action cougar.CougarAction) {
		fmt.Printf("Shard %s changed\n", shardId)
		kcommon.ScheduleRun(1000, func() {
			if action == cougar.CA_AddShard {
				// fmt.Printf("Adding shard %s\n", shardId)
				cougarInstance.VisitState(func(state *cougar.CougarState) string {
					shard := state.AllShards[shardId]
					shard.CurrentConfirmedState = cougarjson.CAS_Ready
					fmt.Printf("Added shard %s\n", shardId)
					return "AddedSucc"
				})
			} else if action == cougar.CA_RemoveShard {
				// fmt.Printf("Removing shard %s\n", shardId)
				cougarInstance.VisitState(func(state *cougar.CougarState) string {
					shard := state.AllShards[shardId]
					shard.CurrentConfirmedState = cougarjson.CAS_Dropped
					fmt.Printf("Removed shard %s\n", shardId)
					return "RemovedSucc"
				})
			} else if action == cougar.CA_UpdateShard {
				// fmt.Printf("Updating shard %s\n", shardId)
				cougarInstance.VisitState(func(state *cougar.CougarState) string {
					fmt.Printf("Updated shard %s\n", shardId)
					return "UpdatedSucc"
				})
			}
		})
	})
	workerInfo := &cougar.WorkerInfo{
		WorkerId:     "worker1",
		SessionId:    "session1",
		AddressPort:  "localhost:8080",
		Capacity:     100,
		MemorySizeMB: 16 * 1024,
		StartTimeMs:  kcommon.GetWallTimeMs(),
	}
	builder.WithWorkerInfo(workerInfo)
	// 创建 Cougar
	cougarInstance = builder.Build(ctx)

	fmt.Println("Cougar created, waiting for state changes...")
	fmt.Scanln()

	fmt.Println("Hello, World!")
}
