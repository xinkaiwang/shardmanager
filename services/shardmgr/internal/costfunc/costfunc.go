package costfunc

import (
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

// var (
// 	currentCostFuncProvider CostFuncProvider
// )

// func GetCurrentCostFuncProvider() CostFuncProvider {
// 	if currentCostFuncProvider == nil {
// 		currentCostFuncProvider = NewCostFuncSimpleProvider()
// 	}
// 	return currentCostFuncProvider
// }

// func RunWithCostFuncProvider(provider CostFuncProvider, fn func()) {
// 	oldProvider := currentCostFuncProvider
// 	currentCostFuncProvider = provider
// 	fn()
// 	currentCostFuncProvider = oldProvider
// }

type CostFuncProvider interface {
	CalCost(snap *Snapshot) Cost
}

// CostFuncSimpleProvider implements the CostFuncProvider interface
type CostFuncSimpleProvider struct {
	CostfuncCfg config.CostfuncConfig
}

func NewCostFuncSimpleProvider(CostfuncCfg config.CostfuncConfig) *CostFuncSimpleProvider {
	return &CostFuncSimpleProvider{
		CostfuncCfg: CostfuncCfg,
	}
}

// CalCost implements the CostFuncProvider interface
func (simple *CostFuncSimpleProvider) CalCost(snap *Snapshot) Cost {
	cost := Cost{HardScore: 0, SoftScore: 0}

	// Part 1: Hard score
	// all those replicas that are not assigned to any worker got 1 point (penalty)
	{
		hard := int32(0)
		snap.AllShards.VisitAll(func(shardId data.ShardId, shard *ShardSnap) {
			for _, replica := range shard.Replicas {
				if !replica.LameDuck && len(replica.Assignments) == 0 {
					hard++
				}
			}
		})
		// for _, shard := range snap.AllShards {
		// 	for _, replica := range shard.Replicas {
		// 		if len(replica.Assignments) == 0 {
		// 			hard++
		// 		}
		// 	}
		// }
		cost.HardScore += hard
	}

	// Part 2: Soft score
	{
		soft := float64(0)
		hard := int32(0)
		snap.AllWorkers.VisitAll(func(workerId data.WorkerFullId, worker *WorkerSnap) {
			soft += float64(len(worker.Assignments)) / float64(simple.CostfuncCfg.ShardCountCostNorm)
			if len(worker.Assignments) > int(simple.CostfuncCfg.WorkerMaxAssignments) {
				hard++
			}
		})
		cost.SoftScore += soft
		cost.HardScore += hard
	}

	cost.SoftScore *= 5000 // this makes soft score more readable
	return cost
}
