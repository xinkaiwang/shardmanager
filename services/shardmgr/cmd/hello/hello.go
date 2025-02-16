package main

import (
	"context"
	"fmt"

	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/etcdprov"
)

func main() {
	fmt.Println("version:" + common.GetVersion())
	ctx := context.Background()
	etcd := etcdprov.GetCurrentEtcdProvider(ctx)
	etcd.Set(ctx, "test", "value")
	ret := etcd.Get(ctx, "test")
	fmt.Println(ret)
}
