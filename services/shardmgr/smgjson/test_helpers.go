package smgjson

import "github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"

// test_helpers.go 包含测试时常用的辅助函数

// CreateTestServiceInfo 创建一个测试用的 ServiceInfoJson 对象
// 使用默认值，适合大多数测试场景
func CreateTestServiceInfo(statefulType data.StatefulType) *ServiceInfoJson {
	return &ServiceInfoJson{
		ServiceName:      "test-service",
		StatefulType:     NewServiceTypePointer(statefulType),
		MoveType:         NewMovePolicyPointer(MP_StartBeforeKill),
		MaxResplicaCount: NewInt32Pointer(10),
		MinResplicaCount: NewInt32Pointer(1),
	}
}

// CreateTestServiceInfoWithOptions 创建一个测试用的 ServiceInfoJson 对象
// 可以自定义各个字段的值
func CreateTestServiceInfoWithOptions(
	serviceName string,
	serviceType data.StatefulType,
	moveType MovePolicy,
	maxReplica int32,
	minReplica int32,
) *ServiceInfoJson {
	return &ServiceInfoJson{
		ServiceName:      serviceName,
		StatefulType:     NewServiceTypePointer(serviceType),
		MoveType:         NewMovePolicyPointer(moveType),
		MaxResplicaCount: NewInt32Pointer(maxReplica),
		MinResplicaCount: NewInt32Pointer(minReplica),
	}
}

// CreateTestServiceConfig 创建一个测试用的 ServiceConfigJson 对象
// 使用默认值，适合大多数测试场景
func CreateTestServiceConfig() *ServiceConfigJson {
	return &ServiceConfigJson{
		ShardConfig: &ShardConfigJson{
			MoveType:        NewMovePolicyPointer(MP_StartBeforeKill),
			MaxReplicaCount: NewInt32Pointer(10),
			MinReplicaCount: NewInt32Pointer(1),
		},
		WorkerConfig: &WorkerConfigJson{
			MaxAssignmentCountPerWorker: NewInt32Pointer(10),
			OfflineGracePeriodSec:       NewInt32Pointer(60),
		},
		SystemLimit: &SystemLimitConfigJson{
			MaxShardsCountLimit:     NewInt32Pointer(1000),
			MaxReplicaCountLimit:    NewInt32Pointer(1000),
			MaxAssignmentCountLimit: NewInt32Pointer(1000),
			MaxHatCountLimit:        NewInt32Pointer(10),
		},
		CostFuncCfg: &CostFuncConfigJson{
			ShardCountCostEnable: NewBoolPointer(true),
			ShardCountCostNorm:   NewInt32Pointer(100),
			WorkerMaxAssignments: NewInt32Pointer(100),
		},
		SolverConfig: &SolverConfigJson{
			SoftSolverConfig: &BaseSolverConfigJson{
				SoftSolverEnabled: NewBoolPointer(true),
				RunPerMinute:      NewInt32Pointer(30),
				ExplorePerRun:     NewInt32Pointer(100),
			},
			AssignSolverConfig: &BaseSolverConfigJson{
				SoftSolverEnabled: NewBoolPointer(true),
				RunPerMinute:      NewInt32Pointer(30),
				ExplorePerRun:     NewInt32Pointer(100),
			},
			UnassignSolverConfig: &BaseSolverConfigJson{
				SoftSolverEnabled: NewBoolPointer(false),
				RunPerMinute:      NewInt32Pointer(30),
				ExplorePerRun:     NewInt32Pointer(100),
			},
		},
	}
}

// ServiceConfigOption 是一个函数类型，用于修改 ServiceConfigJson 对象
type ServiceConfigOption func(*ServiceConfigJson)

// WithMovePolicy 设置 ShardConfig 的移动策略
func WithMovePolicy(movePolicy MovePolicy) ServiceConfigOption {
	return func(cfg *ServiceConfigJson) {
		cfg.ShardConfig.MoveType = NewMovePolicyPointer(movePolicy)
	}
}

// WithReplicaLimits 设置 ShardConfig 的副本数限制
func WithReplicaLimits(min, max int32) ServiceConfigOption {
	return func(cfg *ServiceConfigJson) {
		cfg.ShardConfig.MinReplicaCount = NewInt32Pointer(min)
		cfg.ShardConfig.MaxReplicaCount = NewInt32Pointer(max)
	}
}

// WithSystemLimits 设置系统限制配置
func WithSystemLimits(maxShards, maxReplicas, maxAssignments, maxAssignmentsPerWorker int32) ServiceConfigOption {
	return func(cfg *ServiceConfigJson) {
		cfg.SystemLimit.MaxShardsCountLimit = NewInt32Pointer(maxShards)
		cfg.SystemLimit.MaxReplicaCountLimit = NewInt32Pointer(maxReplicas)
		cfg.SystemLimit.MaxAssignmentCountLimit = NewInt32Pointer(maxAssignments)
	}
}

// CreateTestServiceConfigWithOptions 使用选项模式创建自定义的 ServiceConfigJson
func CreateTestServiceConfigWithOptions(options ...ServiceConfigOption) *ServiceConfigJson {
	cfg := CreateTestServiceConfig()

	// 应用自定义选项
	for _, option := range options {
		option(cfg)
	}

	return cfg
}
