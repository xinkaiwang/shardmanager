package config

import "github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"

type CostfuncConfig struct {
	ShardCountCostEnable bool
	ShardCountCostNorm   int32 // number of shards

	WorkerMaxAssignments int32
}

func CostFuncConfigJsonToConfig(cfc *smgjson.CostFuncConfigJson) CostfuncConfig {
	cfg := CostfuncConfig{
		ShardCountCostEnable: true,
		ShardCountCostNorm:   10,

		WorkerMaxAssignments: 20,
	}

	if cfc == nil {
		return cfg
	}
	if cfc.ShardCountCostEnable != nil {
		cfg.ShardCountCostEnable = *cfc.ShardCountCostEnable
	}
	if cfc.ShardCountCostNorm != nil {
		cfg.ShardCountCostNorm = *cfc.ShardCountCostNorm
	}
	if cfc.WorkerMaxAssignments != nil {
		cfg.WorkerMaxAssignments = *cfc.WorkerMaxAssignments
	}
	return cfg
}
