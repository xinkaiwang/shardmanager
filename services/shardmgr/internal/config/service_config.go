package config

import (
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

func ParseServiceConfigFromJson(data string) *ServiceConfig {
	si := smgjson.ParseServiceConfigFromJson(data)
	return ServiceConfigFromJson(si)
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
	FaultToleranceConfig   FaultToleranceConfig
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
	// MaxConcurrentMoveCountLimit 是每次迁移的最大并发数限制
	MaxConcurrentMoveCountLimit int32
}

type FaultToleranceConfig struct {
	GracePeriodSecBeforeDrain      int32 // once lost eph, we will wait this time to see if the eph is back.
	GracePeriodSecBeforeDirtyPurge int32 // 是在 worker 无法 移除 shard 的情况下，允许 worker 继续存在，不被删除的时间
}

func ServiceConfigFromJson(si *smgjson.ServiceConfigJson) *ServiceConfig {
	return &ServiceConfig{
		ShardConfig:            ShardConfigJsonToConfig(si.ShardConfig),
		WorkerConfig:           WorkerConfigJsonToConfig(si.WorkerConfig),
		SystemLimit:            SystemLimitConfigJsonToConfig(si.SystemLimit),
		CostFuncCfg:            CostFuncConfigJsonToConfig(si.CostFuncCfg),
		SolverConfig:           SolverConfigJsonToConfig(si.SolverConfig),
		DynamicThresholdConfig: DynamicThresholdConfigJsonToConfig(si.DynamicThresholdConfig),
		FaultToleranceConfig:   FaultToleranceConfigJsonToConfig(si.FaultToleranceConfig),
	}
}

func (cfg *ServiceConfig) ToJsonObj() *smgjson.ServiceConfigJson {
	return &smgjson.ServiceConfigJson{
		ShardConfig:            cfg.ShardConfig.ToJsonObj(),
		WorkerConfig:           cfg.WorkerConfig.ToJsonObj(),
		SystemLimit:            cfg.SystemLimit.ToJsonObj(),
		CostFuncCfg:            cfg.CostFuncCfg.ToJsonObj(),
		SolverConfig:           cfg.SolverConfig.ToJsonObj(),
		DynamicThresholdConfig: cfg.DynamicThresholdConfig.ToJsonObj(),
		FaultToleranceConfig:   cfg.FaultToleranceConfig.ToJsonObj(),
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
		MaxShardsCountLimit:         1000, // default 1000
		MaxReplicaCountLimit:        1000, // default 1000
		MaxAssignmentCountLimit:     1000, // default 1000
		MaxHatCountLimit:            10,   // default 10
		MaxConcurrentMoveCountLimit: 30,   // default 30
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
	if sc.MaxConcurrentMoveCountLimit != nil {
		cfg.MaxConcurrentMoveCountLimit = *sc.MaxConcurrentMoveCountLimit
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

func FaultToleranceConfigJsonToConfig(sc *smgjson.FaultToleranceConfigJson) FaultToleranceConfig {
	cfg := FaultToleranceConfig{
		GracePeriodSecBeforeDrain:      0,         // default 0 sec
		GracePeriodSecBeforeDirtyPurge: 24 * 3600, // default 24h
	}
	if sc == nil {
		return cfg
	}
	if sc.GracePeriodSecBeforeDrain != nil {
		cfg.GracePeriodSecBeforeDrain = *sc.GracePeriodSecBeforeDrain
	}
	if sc.GracePeriodSecBeforeDirtyPurge != nil {
		cfg.GracePeriodSecBeforeDirtyPurge = *sc.GracePeriodSecBeforeDirtyPurge
	}
	return cfg
}

func (cfg *ServiceConfig) ToServiceConfigJson() *smgjson.ServiceConfigJson {
	return &smgjson.ServiceConfigJson{
		ShardConfig:            cfg.ShardConfig.ToJsonObj(),
		WorkerConfig:           cfg.WorkerConfig.ToJsonObj(),
		SystemLimit:            cfg.SystemLimit.ToJsonObj(),
		CostFuncCfg:            cfg.CostFuncCfg.ToJsonObj(),
		SolverConfig:           cfg.SolverConfig.ToJsonObj(),
		DynamicThresholdConfig: cfg.DynamicThresholdConfig.ToJsonObj(),
		FaultToleranceConfig:   cfg.FaultToleranceConfig.ToJsonObj(),
	}
}
func (cfg *ShardConfig) ToJsonObj() *smgjson.ShardConfigJson {
	return &smgjson.ShardConfigJson{
		MoveType:        &cfg.MovePolicy,
		MaxReplicaCount: func() *int32 { i := int32(cfg.MaxReplicaCount); return &i }(),
		MinReplicaCount: func() *int32 { i := int32(cfg.MinReplicaCount); return &i }(),
	}
}
func (cfg *WorkerConfig) ToJsonObj() *smgjson.WorkerConfigJson {
	return &smgjson.WorkerConfigJson{
		MaxAssignmentCountPerWorker: &cfg.MaxAssignmentCountPerWorker,
		OfflineGracePeriodSec:       &cfg.OfflineGracePeriodSec,
	}
}
func (cfg *SystemLimitConfig) ToJsonObj() *smgjson.SystemLimitConfigJson {
	return &smgjson.SystemLimitConfigJson{
		MaxShardsCountLimit:         &cfg.MaxShardsCountLimit,
		MaxReplicaCountLimit:        &cfg.MaxReplicaCountLimit,
		MaxAssignmentCountLimit:     &cfg.MaxAssignmentCountLimit,
		MaxHatCountLimit:            &cfg.MaxHatCountLimit,
		MaxConcurrentMoveCountLimit: &cfg.MaxConcurrentMoveCountLimit,
	}
}
func (cfg *DynamicThresholdConfig) ToJsonObj() *smgjson.DynamicThresholdConfigJson {
	return &smgjson.DynamicThresholdConfigJson{
		DynamicThresholdMax: &cfg.DynamicThresholdMax,
		DynamicThresholdMin: &cfg.DynamicThresholdMin,
		HalfDecayTimeSec:    &cfg.HalfDecayTimeSec,
		IncreasePerMove:     &cfg.IncreasePerMove,
	}
}
func (cfg *FaultToleranceConfig) ToJsonObj() *smgjson.FaultToleranceConfigJson {
	return &smgjson.FaultToleranceConfigJson{
		GracePeriodSecBeforeDrain:      &cfg.GracePeriodSecBeforeDrain,
		GracePeriodSecBeforeDirtyPurge: &cfg.GracePeriodSecBeforeDirtyPurge,
	}
}
