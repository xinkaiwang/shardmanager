package main

import (
	"context"
	"fmt"

	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/etcdprov"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

func main() {
	fmt.Println("version:" + common.GetVersion())
	// ctx := context.Background()
	// testEtcd(ctx)
	printCfg()
}

func printCfg() {
	cfg := config.ServiceConfigFromJson(&smgjson.ServiceConfigJson{})
	str := cfg.ToJsonObj().ToJson()
	fmt.Println(str)
}

func testEtcd(ctx context.Context) {
	etcd := etcdprov.GetCurrentEtcdProvider(ctx)
	etcd.Set(ctx, "test", "value")
	ret := etcd.Get(ctx, "test")
	fmt.Println(ret)
}
