package cougarapp

import "github.com/xinkaiwang/shardmanager/libs/cougar/cougar"

type CougarApp interface {
	AddShard(shard cougar.CougarShard) AppShard
}

type AppShard interface {
	HandleRequest(method string, id string, reqBody interface{}) interface{}
}
