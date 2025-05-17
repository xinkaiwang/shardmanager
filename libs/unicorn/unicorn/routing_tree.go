package unicorn

import "github.com/xinkaiwang/shardmanager/libs/unicorn/data"

type RoutingTree interface {
	GetTreeId() string // for logging/debugging use only
	FindShardByShardingKey(shardingKey data.ShardingKey) *RoutingTarget
}

type RoutingTarget struct {
	ShardId    data.ShardId
	WorkerInfo *WorkerInfo
}
