package config

import "github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"

type ServiceConfigOption func(*ServiceConfig)

func WithMovePolicy(movePolicy smgjson.MovePolicy) ServiceConfigOption {
	return func(cfg *ServiceConfig) {
		cfg.ShardConfig.MovePolicy = movePolicy
	}
}

func WithReplicaLimits(min, max int32) ServiceConfigOption {
	return func(cfg *ServiceConfig) {
		cfg.ShardConfig.MinReplicaCount = int(min)
		cfg.ShardConfig.MaxReplicaCount = int(max)
	}
}

// WithSystemLimits 设置系统限制配置
func WithSystemLimits(maxShards, maxReplicas, maxAssignments, maxAssignmentsPerWorker int32) ServiceConfigOption {
	return func(cfg *ServiceConfig) {
		cfg.SystemLimit.MaxShardsCountLimit = maxShards
		cfg.SystemLimit.MaxReplicaCountLimit = maxReplicas
		cfg.SystemLimit.MaxAssignmentCountLimit = maxAssignments
	}
}

func WithSolverConfig(fn func(*SolverConfig)) ServiceConfigOption {
	return func(cfg *ServiceConfig) {
		fn(&cfg.SolverConfig)
	}
}

func WithShardConfig(fn func(*ShardConfig)) ServiceConfigOption {
	return func(cfg *ServiceConfig) {
		fn(&cfg.ShardConfig)
	}
}

func WithOfflineGracePeriodSec(seconds int32) ServiceConfigOption {
	return func(cfg *ServiceConfig) {
		cfg.WorkerConfig.OfflineGracePeriodSec = seconds
	}
}

func CreateTestServiceConfig(options ...ServiceConfigOption) *ServiceConfig {
	cfg := ServiceConfigFromJson(&smgjson.ServiceConfigJson{}) // config (with all default values)
	for _, option := range options {
		option(cfg)
	}
	return cfg
}
