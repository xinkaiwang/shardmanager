package smgjson

import (
	"encoding/json"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
)

type ServiceConfigJson struct {
	ShardConfig *ShardConfigJson       `json:"shard_config"`
	SystemLimit *SystemLimitConfigJson `json:"system_limit"`
	CostFuncCfg *CostFuncConfigJson    `json:"cost_func_cfg"`
}

type ShardConfigJson struct {
	// MoveType 服务的迁移类型 (killBeforeStart, startBeforeKill, concurrent)
	MoveType *MovePolicy `json:"move_policy"`
	// max/min replica count per shard (can be override by per shard config)
	MaxResplicaCount *int32 `json:"max_replica_count"` // default max replica count per shard (default 10)
	MinResplicaCount *int32 `json:"min_replica_count"` // default min replica count per shard (default 1)
}

type SystemLimitConfigJson struct {
	// MaxShardsCountLimit 是 shard 的最大数量限制
	MaxShardsCountLimit *int32 `json:"max_shards_count_limit"`
	// MaxReplicaCountLimit 是 shard 的最大副本数限制
	MaxReplicaCountLimit *int32 `json:"max_replica_count_limit"`
	// MaxAssignmentCountLimit 是 shard 的最大 assignment 数量限制
	MaxAssignmentCountLimit *int32 `json:"max_assignment_count_limit"`
	// MaxAssignmentCount 是 per worker 的最大 assignment 数量限制
	MaxAssignmentCountPerWorker *int32 `json:"max_assignment_count_per_worker"`
}

type CostFuncConfigJson struct {
	QpmNormFactor *float64 `json:"qpm_norm_factor"`
	CpuNormFactor *float64 `json:"cpu_norm_factor"`
	MemNormFactor *float64 `json:"mem_norm_factor"`
}

func (sc *ServiceConfigJson) ToJson() string {
	data, err := json.Marshal(sc)
	if err != nil {
		ke := kerror.Wrap(err, "MarshalError", "failed to marshal ServiceConfigJson", false)
		panic(ke)
	}
	return string(data)
}
