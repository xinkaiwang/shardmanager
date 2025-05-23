package smgjson

import (
	"encoding/json"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

type ShardStateJson struct {
	// ShardName
	ShardName data.ShardId `json:"shard_name"`

	// key is replicaIdx
	Resplicas map[data.ReplicaIdx]*ReplicaStateJson `json:"resplicas"`

	TargetReplicaCount int `json:"target_replica_count,omitempty"`

	LameDuck int8 `json:"lame_duck,omitempty"` // use int to represent bool

	CustomProperties map[string]string `json:"custom_properties,omitempty"`

	LastUpdateTimeMs int64 `json:"last_update_time_ms,omitempty"` // last update time in ms

	LastUpdateReason string `json:"last_update_reason,omitempty"`
}

func NewShardStateJson(shardName data.ShardId) *ShardStateJson {
	return &ShardStateJson{
		ShardName: shardName,
		Resplicas: make(map[data.ReplicaIdx]*ReplicaStateJson),
	}
}

func (obj *ShardStateJson) ToJson() string {
	bytes, err := json.Marshal(obj)
	if err != nil {
		ke := kerror.Wrap(err, "MarshalError", "failed to marshal ShardStateJson", false)
		panic(ke)
	}
	return string(bytes)
}

func ShardStateJsonFromJson(stringJson string) *ShardStateJson {
	var obj ShardStateJson
	err := json.Unmarshal([]byte(stringJson), &obj)
	if err != nil {
		ke := kerror.Wrap(err, "UnmarshalError", "failed to unmarshal ShardStateJson", false)
		panic(ke)
	}

	// Validate required fields
	if obj.ShardName == "" {
		panic(kerror.Create("UnmarshalError", "missing required field: shard_name"))
	}
	if obj.Resplicas == nil {
		obj.Resplicas = make(map[data.ReplicaIdx]*ReplicaStateJson)
	}
	return &obj
}
