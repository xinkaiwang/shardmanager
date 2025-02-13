package main

import (
	"context"
	"fmt"

	"github.com/xinkaiwang/shardmanager/services/etcdmgr/internal/provider"
)

func main() {
	ctx := context.Background()
	etcd := provider.GetCurrentEtcdProvider(ctx)
	etcd.Set(ctx, "test", "value")
	ret := etcd.Get(ctx, "test")
	fmt.Println(ret)
}
