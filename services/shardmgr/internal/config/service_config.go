package config

import (
	"encoding/json"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

func ParseServiceConfigFromJson(data string) *ServiceConfig {
	si := &smgjson.ServiceConfigJson{}
	if data != "" {
		err := json.Unmarshal([]byte(data), si)
		if err != nil {
			ke := kerror.Wrap(err, "UnmarshalError", "failed to unmarshal ServiceConfigJson", false)
			panic(ke)
		}
	}
	return ServiceConfigJsonToServiceConfig(si)
}

type ServiceConfig struct {
	ShardConfig  ShardConfig
	SystemLimit  SystemLimitConfig
	CostFuncCfg  CostfuncConfig
	SolverConfig SolverConfig
}

type ShardConfig struct {
	// MoveType 服务的迁移类型 (killBeforeStart, startBeforeKill, concurrent)
	MovePolicy smgjson.MovePolicy
	// max/min replica count per shard (can be override by per shard config)
	MaxResplicaCount int32 // default max replica count per shard (default 10)
	MinResplicaCount int32 // default min replica count per shard (default 1)
}

type SystemLimitConfig struct {
	// MaxShardsCountLimit 是 shard 的最大数量限制
	MaxShardsCountLimit int32
	// MaxReplicaCountLimit 是 shard 的最大副本数限制
	MaxReplicaCountLimit int32
	// MaxAssignmentCountLimit 是 shard 的最大 assignment 数量限制
	MaxAssignmentCountLimit int32
	// MaxAssignmentCount 是 per worker 的最大 assignment 数量限制
	MaxAssignmentCountPerWorker int32
}

func ServiceConfigJsonToServiceConfig(si *smgjson.ServiceConfigJson) *ServiceConfig {
	return &ServiceConfig{
		ShardConfig:  ShardConfigJsonToConfig(si.ShardConfig),
		SystemLimit:  SystemLimitConfigJsonToConfig(si.SystemLimit),
		CostFuncCfg:  CostFuncConfigJsonToConfig(si.CostFuncCfg),
		SolverConfig: SolverConfigJsonToConfig(si.SolverConfig),
	}
}

func ShardConfigJsonToConfig(sc *smgjson.ShardConfigJson) ShardConfig {
	cfg := ShardConfig{
		MovePolicy:       smgjson.MP_StartBeforeKill, // default 先启后杀
		MaxResplicaCount: 10,                         // default 10
		MinResplicaCount: 1,                          // default 1

	}
	if sc == nil {
		return cfg
	}
	if sc.MoveType != nil {
		cfg.MovePolicy = *sc.MoveType
	}
	if sc.MaxResplicaCount != nil {
		cfg.MaxResplicaCount = *sc.MaxResplicaCount
	}
	if sc.MinResplicaCount != nil {
		cfg.MinResplicaCount = *sc.MinResplicaCount
	}
	return cfg
}

func SystemLimitConfigJsonToConfig(sc *smgjson.SystemLimitConfigJson) SystemLimitConfig {
	cfg := SystemLimitConfig{
		MaxShardsCountLimit:         1000, // default 1000
		MaxReplicaCountLimit:        1000, // default 1000
		MaxAssignmentCountLimit:     1000, // default 1000
		MaxAssignmentCountPerWorker: 100,  // default 100
	}
	if sc == nil {
		return cfg
	}
	if sc.MaxShardsCountLimit != nil {
		cfg.MaxShardsCountLimit = *sc.MaxShardsCountLimit
	}
	if sc.MaxReplicaCountLimit != nil {
		cfg.MaxReplicaCountLimit = *sc.MaxReplicaCountLimit
	}
	if sc.MaxAssignmentCountLimit != nil {
		cfg.MaxAssignmentCountLimit = *sc.MaxAssignmentCountLimit
	}
	if sc.MaxAssignmentCountPerWorker != nil {
		cfg.MaxAssignmentCountPerWorker = *sc.MaxAssignmentCountPerWorker
	}
	return cfg
}

func (cfg *ServiceConfig) ToServiceConfigJson() *smgjson.ServiceConfigJson {
	return &smgjson.ServiceConfigJson{
		ShardConfig: cfg.ShardConfig.ToShardConfigJson(),
		SystemLimit: cfg.SystemLimit.ToSystemLimitConfigJson(),
	}
}
func (cfg *ShardConfig) ToShardConfigJson() *smgjson.ShardConfigJson {
	return &smgjson.ShardConfigJson{
		MoveType:         &cfg.MovePolicy,
		MaxResplicaCount: &cfg.MaxResplicaCount,
		MinResplicaCount: &cfg.MinResplicaCount,
	}
}
func (cfg *SystemLimitConfig) ToSystemLimitConfigJson() *smgjson.SystemLimitConfigJson {
	return &smgjson.SystemLimitConfigJson{
		MaxShardsCountLimit:         &cfg.MaxShardsCountLimit,
		MaxReplicaCountLimit:        &cfg.MaxReplicaCountLimit,
		MaxAssignmentCountLimit:     &cfg.MaxAssignmentCountLimit,
		MaxAssignmentCountPerWorker: &cfg.MaxAssignmentCountPerWorker,
	}
}
