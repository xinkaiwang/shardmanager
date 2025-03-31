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

type ServiceConfigProvider interface {
	GetServiceConfig() *ServiceConfig
}

type ServiceConfig struct {
	ShardConfig            ShardConfig
	WorkerConfig           WorkerConfig
	SystemLimit            SystemLimitConfig
	CostFuncCfg            CostfuncConfig
	SolverConfig           SolverConfig
	DynamicThresholdConfig DynamicThresholdConfig
}

type ShardConfig struct {
	// MoveType 服务的迁移类型 (killBeforeStart, startBeforeKill, concurrent)
	MovePolicy smgjson.MovePolicy
	// max/min replica count per shard (can be override by per shard config)
	MaxReplicaCount int // default max replica count per shard (default 10)
	MinReplicaCount int // default min replica count per shard (default 1)
}

type WorkerConfig struct {
	// MaxAssignmentCountPerWorker 是 per worker 的最大 assignment 数量限制
	MaxAssignmentCountPerWorker int32
	// Offline Grace Period sec 是 worker 下线的宽限期
	OfflineGracePeriodSec int32
}

type SystemLimitConfig struct {
	// MaxShardsCountLimit 是 shard 的最大数量限制
	MaxShardsCountLimit int32
	// MaxReplicaCountLimit 是 shard 的最大副本数限制
	MaxReplicaCountLimit int32
	// MaxAssignmentCountLimit 是 shard 的最大 assignment 数量限制
	MaxAssignmentCountLimit int32
	// MaxHatCount
	MaxHatCountLimit int32
}

func ServiceConfigJsonToServiceConfig(si *smgjson.ServiceConfigJson) *ServiceConfig {
	return &ServiceConfig{
		ShardConfig:            ShardConfigJsonToConfig(si.ShardConfig),
		WorkerConfig:           WorkerConfigJsonToConfig(si.WorkerConfig),
		SystemLimit:            SystemLimitConfigJsonToConfig(si.SystemLimit),
		CostFuncCfg:            CostFuncConfigJsonToConfig(si.CostFuncCfg),
		SolverConfig:           SolverConfigJsonToConfig(si.SolverConfig),
		DynamicThresholdConfig: DynamicThresholdConfigJsonToConfig(si.DynamicThresholdConfig),
	}
}

func ShardConfigJsonToConfig(sc *smgjson.ShardConfigJson) ShardConfig {
	cfg := ShardConfig{
		MovePolicy:      smgjson.MP_StartBeforeKill, // default 先启后杀
		MaxReplicaCount: 10,                         // default 10
		MinReplicaCount: 1,                          // default 1

	}
	if sc == nil {
		return cfg
	}
	if sc.MoveType != nil {
		cfg.MovePolicy = *sc.MoveType
	}
	if sc.MaxReplicaCount != nil {
		cfg.MaxReplicaCount = int(*sc.MaxReplicaCount)
	}
	if sc.MinReplicaCount != nil {
		cfg.MinReplicaCount = int(*sc.MinReplicaCount)
	}
	return cfg
}

func WorkerConfigJsonToConfig(wc *smgjson.WorkerConfigJson) WorkerConfig {
	cfg := WorkerConfig{
		MaxAssignmentCountPerWorker: 100, // default 100
		OfflineGracePeriodSec:       10,  // default 10 sec
	}
	if wc == nil {
		return cfg
	}
	if wc.MaxAssignmentCountPerWorker != nil {
		cfg.MaxAssignmentCountPerWorker = *wc.MaxAssignmentCountPerWorker
	}
	if wc.OfflineGracePeriodSec != nil {
		cfg.OfflineGracePeriodSec = *wc.OfflineGracePeriodSec
	}
	return cfg
}

func SystemLimitConfigJsonToConfig(sc *smgjson.SystemLimitConfigJson) SystemLimitConfig {
	cfg := SystemLimitConfig{
		MaxShardsCountLimit:     1000, // default 1000
		MaxReplicaCountLimit:    1000, // default 1000
		MaxAssignmentCountLimit: 1000, // default 1000
		MaxHatCountLimit:        10,   // default 10
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
	if sc.MaxHatCountLimit != nil {
		cfg.MaxHatCountLimit = *sc.MaxHatCountLimit
	}
	return cfg
}

func DynamicThresholdConfigJsonToConfig(sc *smgjson.DynamicThresholdConfigJson) DynamicThresholdConfig {
	cfg := DynamicThresholdConfig{
		DynamicThresholdMax: 100, // default 100
		DynamicThresholdMin: 10,  // default 10
		HalfDecayTimeSec:    300, // default 5 min
		IncreasePerMove:     10,  // default 10
	}
	if sc == nil {
		return cfg
	}
	if sc.DynamicThresholdMax != nil {
		cfg.DynamicThresholdMax = *sc.DynamicThresholdMax
	}
	if sc.DynamicThresholdMin != nil {
		cfg.DynamicThresholdMin = *sc.DynamicThresholdMin
	}
	if sc.HalfDecayTimeSec != nil {
		cfg.HalfDecayTimeSec = *sc.HalfDecayTimeSec
	}
	if sc.IncreasePerMove != nil {
		cfg.IncreasePerMove = *sc.IncreasePerMove
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
		MoveType:        &cfg.MovePolicy,
		MaxReplicaCount: func() *int32 { i := int32(cfg.MaxReplicaCount); return &i }(),
		MinReplicaCount: func() *int32 { i := int32(cfg.MinReplicaCount); return &i }(),
	}
}
func (cfg *WorkerConfig) ToWorkerConfigJson() *smgjson.WorkerConfigJson {
	return &smgjson.WorkerConfigJson{
		MaxAssignmentCountPerWorker: &cfg.MaxAssignmentCountPerWorker,
		OfflineGracePeriodSec:       &cfg.OfflineGracePeriodSec,
	}
}
func (cfg *SystemLimitConfig) ToSystemLimitConfigJson() *smgjson.SystemLimitConfigJson {
	return &smgjson.SystemLimitConfigJson{
		MaxShardsCountLimit:     &cfg.MaxShardsCountLimit,
		MaxReplicaCountLimit:    &cfg.MaxReplicaCountLimit,
		MaxAssignmentCountLimit: &cfg.MaxAssignmentCountLimit,
	}
}
func (cfg *DynamicThresholdConfig) ToDynamicThresholdConfigJson() *smgjson.DynamicThresholdConfigJson {
	return &smgjson.DynamicThresholdConfigJson{
		DynamicThresholdMax: &cfg.DynamicThresholdMax,
		DynamicThresholdMin: &cfg.DynamicThresholdMin,
		HalfDecayTimeSec:    &cfg.HalfDecayTimeSec,
		IncreasePerMove:     &cfg.IncreasePerMove,
	}
}
