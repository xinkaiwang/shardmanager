package smgjson

import (
	"encoding/json"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
)

type ServiceConfigJson struct {
	ShardConfig  *ShardConfigJson       `json:"shard_config"`
	SystemLimit  *SystemLimitConfigJson `json:"system_limit"`
	CostFuncCfg  *CostFuncConfigJson    `json:"cost_func_cfg"`
	SolverConfig *SolverConfigJson      `json:"solver_config"`
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
	ShardCountCostEnable *bool  `json:"shard_count_cost_enable"`
	ShardCountCostNorm   *int32 `json:"shard_count_cost_norm"`

	WorkerMaxAssignments *int32 `json:"worker_max_assignments"`
}

type SolverConfigJson struct {
	SoftSolverConfig     *SoftSolverConfigJson     `json:"soft_solver_config"`
	AssignSolverConfig   *AssignSolverConfigJson   `json:"assign_solver_config"`
	UnassignSolverConfig *UnassignSolverConfigJson `json:"unassign_solver_config"`
}

type SoftSolverConfigJson struct {
	SoftSolverEnabled *bool  `json:"soft_solver_enabled"`
	RunPerMinute      *int32 `json:"run_per_minute"`
	ExplorePerRun     *int32 `json:"explore_per_run"`
}

type AssignSolverConfigJson struct {
	AssignSolverEnabled *bool  `json:"assign_solver_enabled"`
	RunPerMinute        *int32 `json:"run_per_minute"`
	ExplorePerRun       *int32 `json:"explore_per_run"`
}

type UnassignSolverConfigJson struct {
	UnassignSolverEnabled *bool  `json:"unassign_solver_enabled"`
	RunPerMinute          *int32 `json:"run_per_minute"`
	ExplorePerRun         *int32 `json:"explore_per_run"`
}

func (sc *ServiceConfigJson) ToJson() string {
	data, err := json.Marshal(sc)
	if err != nil {
		ke := kerror.Wrap(err, "MarshalError", "failed to marshal ServiceConfigJson", false)
		panic(ke)
	}
	return string(data)
}
