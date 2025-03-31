package smgjson

import (
	"encoding/json"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

// etcd path is /smg/config/service_info.json
/*
{
	"service_name": "shardmgr",
	"stateful_type": "soft_stateful",
	"move_type": "start_before_kill",
	"max_replica_count": 10,
	"min_replica_count": 1
}
*/
type ServiceInfoJson struct {
	// ServiceName 是服务的名称
	ServiceName string `json:"service_name"`

	// StatefulType 服务的类型 (stateless, softStateful, hardStateful)
	StatefulType *data.StatefulType `json:"stateful_type"`

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

func ParseServiceInfoJson(data string) *ServiceInfoJson {
	si := &ServiceInfoJson{}
	err := json.Unmarshal([]byte(data), si)
	if err != nil {
		ke := kerror.Wrap(err, "UnmarshalError", "failed to unmarshal ServiceInfoJson", false)
		panic(ke)
	}
	return si
}

func (si *ServiceInfoJson) ToJson() string {
	data, err := json.Marshal(si)
	if err != nil {
		ke := kerror.Wrap(err, "MarshalError", "failed to marshal ServiceInfoJson", false)
		panic(ke)
	}
	return string(data)
}
