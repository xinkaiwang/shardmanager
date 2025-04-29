package main

import (
	"context"
	"fmt"

	"github.com/xinkaiwang/shardmanager/libs/cougar/etcdprov"
)

func main() {
	// 显示版本信息
	ctx := context.Background()
	println("Etcd Demo")

	// 1. 创建 EtcdProvider 实例 (如果失败会 panic)
	provider := etcdprov.GetCurrentEtcdProvider(ctx)

	// 2. 使用 provider 创建 session (如果失败会 panic)
	session := provider.CreateEtcdSession(ctx)

	fmt.Printf("Etcd session created successfully. State: %s\n", session.GetCurrentState())

	session.PutNode("/test/eph1", "value1")

	ch := session.WatchByPrefix(ctx, "/test/pilot1", 0)
	go func() {
		for {
			item, ok := <-ch
			if !ok {
				fmt.Println("WatchByPrefix Channel closed")
				break
			}
			fmt.Printf("Received event: %s, value: %s\n", item.Key, item.Value)
		}
	}()

	// wait until user press enter
	fmt.Println("Press Enter to exit")
	fmt.Scanln()
	session.DeleteNode("/test/eph1")
	fmt.Println("Node deleted")
	// 3. 关闭 session
	session.Close(ctx)
	fmt.Println("Session closed")
}
