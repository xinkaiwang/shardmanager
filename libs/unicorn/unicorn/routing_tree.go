package unicorn

import (
	"encoding/json"

	"github.com/xinkaiwang/shardmanager/libs/unicorn/data"
)

type RoutingTree interface {
	GetTreeId() string // for logging/debugging use only
	FindShardByShardingKey(shardingKey data.ShardingKey) *RoutingTarget
}

type RoutingTarget struct {
	ShardId    data.ShardId
	WorkerInfo *WorkerInfo
}

func (rt *RoutingTarget) String() string {
	data, err := json.Marshal(rt)
	if err != nil {
		panic(err)
	}
	return string(data)
}
