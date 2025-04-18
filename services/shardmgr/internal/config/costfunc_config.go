package config

import (
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

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
		cfg.ShardCountCostEnable = common.BoolFromInt8(*cfc.ShardCountCostEnable)
	}
	if cfc.ShardCountCostNorm != nil {
		cfg.ShardCountCostNorm = *cfc.ShardCountCostNorm
	}
	if cfc.WorkerMaxAssignments != nil {
		cfg.WorkerMaxAssignments = *cfc.WorkerMaxAssignments
	}
	return cfg
}

func (cfc *CostfuncConfig) ToJsonObj() *smgjson.CostFuncConfigJson {
	cfg := &smgjson.CostFuncConfigJson{
		ShardCountCostEnable: nil,
		ShardCountCostNorm:   nil,
		WorkerMaxAssignments: nil,
	}
	if cfc.ShardCountCostEnable {
		cfg.ShardCountCostEnable = smgjson.NewInt8Pointer(common.Int8FromBool(cfc.ShardCountCostEnable))
	}
	if cfc.ShardCountCostNorm > 0 {
		intVal := int32(cfc.ShardCountCostNorm)
		cfg.ShardCountCostNorm = &intVal
	}
	if cfc.WorkerMaxAssignments > 0 {
		intVal := int32(cfc.WorkerMaxAssignments)
		cfg.WorkerMaxAssignments = &intVal
	}
	return cfg
}
