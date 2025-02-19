package smgjson

import (
	"encoding/json"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
)

// etcd path is /smg/config/service_info.json
/*
{
	"service_name": "shardmgr",
	"service_type": "soft_stateful",
	"move_type": "start_before_kill",
	"max_replica_count": 10,
	"min_replica_count": 1
}
*/
type ServiceInfoJson struct {
	// ServiceName 是服务的名称
	ServiceName string `json:"service_name"`

	// ServiceType 服务的类型 (stateless, softStateful, hardStateful)
	ServiceType *ServiceType `json:"service_type"`

	// MoveType 服务的迁移类型 (killBeforeStart, startBeforeKill, concurrent)
	MoveType *MovePolicy `json:"move_type"`

	// max/min replica count per shard (can be override by per shard config)
	MaxResplicaCount *int32 `json:"max_replica_count"` // default max replica count per shard (default 10)
	MinResplicaCount *int32 `json:"min_replica_count"` // default min replica count per shard (default 1)
}

// MovePolicy 迁移策略, 用于描述迁移 assignment 时的操作顺序.
type MovePolicy string

const (
	MP_KillBeforeStart MovePolicy = "kill_before_start" // 先杀后启
	MP_StartBeforeKill MovePolicy = "start_before_kill" // 先启后杀 (default)
	MP_Cocurrent       MovePolicy = "concurrent"        // 同时进行
)

type ServiceType string

const (
	ST_STATELESS     ServiceType = "stateless"     // 无状态，无需热身。迁移成本为0
	ST_SOFT_STATEFUL ServiceType = "soft_stateful" // 弱状态，状态存在于内存。热身需要数秒,若掉电后需要重新热身。
	ST_HARD_STATEFUL ServiceType = "hard_stateful" // 强状态，状态存在于硬盘。热身需要数分钟/小时,但掉电重启后不需要重新热身
)

func ParseServiceInfoJson(data string) *ServiceInfoJson {
	si := &ServiceInfoJson{}
	err := json.Unmarshal([]byte(data), si)
	if err != nil {
		ke := kerror.Wrap(err, "UnmarshalError", "failed to unmarshal ServiceInfoJson", false)
		panic(ke)
	}
	return si
}
