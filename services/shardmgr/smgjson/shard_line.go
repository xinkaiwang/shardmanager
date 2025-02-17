package smgjson

import (
	"encoding/json"
	"strings"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
)

/* shar line can looks like this:
<name only>
shard_1200_1300
or
<name + hints>
shard_1200_1300|{min_replica_count:1,max_replica_count:10}
or
<name + hints + custom_properties>
shard_1200_1300|{min_replica_count:1,max_replica_count:10}|{"pip":"xformer","custom_node":"node1"}
*/

type ShardLineJson struct {
	// ShardName 是 shard 的名称
	ShardName string

	Hints *ShardHintsJson

	// CustomProperties 是 shard 的自定义属性
	CustomProperties map[string]string
}

func NewShardLineJson(shardName string) *ShardLineJson {
	return &ShardLineJson{
		ShardName: shardName,
	}
}

func (sl *ShardLineJson) ToShardLine(defVal ShardHints) *ShardLine {
	return &ShardLine{
		ShardName:        sl.ShardName,
		Hints:            sl.GetHints(defVal),
		CustomProperties: sl.CustomProperties,
	}
}

func (sl *ShardLineJson) GetHints(defVal ShardHints) ShardHints {
	if sl.Hints == nil {
		return defVal
	}

	hints := ShardHints{}
	// MinReplicaCount (如果 hints 为空，则使用默认值)
	if sl.Hints.MinReplicaCount != nil {
		hints.MinReplicaCount = *sl.Hints.MinReplicaCount
	} else {
		hints.MinReplicaCount = defVal.MinReplicaCount
	}
	// MaxReplicaCount (如果 hints 为空，则使用默认值)
	if sl.Hints.MaxReplicaCount != nil {
		hints.MaxReplicaCount = *sl.Hints.MaxReplicaCount
	} else {
		hints.MaxReplicaCount = defVal.MaxReplicaCount
	}
	// MoveType (如果 hints 为空，则使用默认值)
	if sl.Hints.MoveType != nil {
		hints.MoveType = *sl.Hints.MoveType
	} else {
		hints.MoveType = defVal.MoveType
	}
	return hints
}

type ShardHintsJson struct {
	// MinReplicaCount 是 shard 的最小副本数
	MinReplicaCount *int32 `json:"min_replica_count"`

	// MaxReplicaCount 是 shard 的最大副本数
	MaxReplicaCount *int32 `json:"max_replica_count"`

	// MoveType 是 shard 的迁移类型
	MoveType *MovePolicy `json:"move_type"`
}

func ShardHintsJsonFromJson(stringJson string) *ShardHintsJson {
	var obj ShardHintsJson
	err := json.Unmarshal([]byte(stringJson), &obj)
	if err != nil {
		ke := kerror.Wrap(err, "UnmarshalError", "failed to unmarshal ShardHintsJson", false)
		panic(ke)
	}
	return &obj
}

type ShardHints struct {
	// MinReplicaCount 是 shard 的最小副本数
	MinReplicaCount int32

	// MaxReplicaCount 是 shard 的最大副本数
	MaxReplicaCount int32

	// MoveType 是 shard 的迁移类型
	MoveType MovePolicy
}

func ParseShardLine(shardLine string) *ShardLineJson {
	parts := strings.SplitN(shardLine, "|", 3)

	// 解析 shardName
	var shardName, hintsStr, customPropertiesStr string
	if len(parts) == 1 {
		shardName = strings.TrimRight(parts[0], "\r")
	} else if len(parts) == 2 {
		shardName = strings.TrimRight(parts[0], "\r")
		hintsStr = parts[1]
	} else if len(parts) == 3 {
		shardName = strings.TrimRight(parts[0], "\r")
		hintsStr = parts[1]
		customPropertiesStr = parts[2]
	} else {
		ke := kerror.Create("InvalidShardLine", "invalid shard line format")
		panic(ke)
	}

	// 解析 hints
	var hints *ShardHintsJson
	if hintsStr != "" {
		hints = ShardHintsJsonFromJson(hintsStr)
	}
	// 解析 customProperties
	customProperties := make(map[string]string)
	if customPropertiesStr != "" {
		customProperties = ParseCustomProperties(customPropertiesStr)
	}
	// 返回 ShardLine
	return &ShardLineJson{
		ShardName:        shardName,
		Hints:            hints,
		CustomProperties: customProperties,
	}
}

func ParseCustomProperties(customPropertiesStr string) map[string]string {
	customProperties := make(map[string]string)
	if customPropertiesStr != "" {
		// 解析 customProperties
		err := json.Unmarshal([]byte(customPropertiesStr), &customProperties)
		if err != nil {
			ke := kerror.Wrap(err, "UnmarshalError", "failed to unmarshal customProperties", false)
			panic(ke)
		}
	}
	return customProperties
}

type ShardLine struct {
	// ShardName 是 shard 的名称
	ShardName string

	Hints ShardHints

	// CustomProperties 是 shard 的自定义属性
	CustomProperties map[string]string
}
