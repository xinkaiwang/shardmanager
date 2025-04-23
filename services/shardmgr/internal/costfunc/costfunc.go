package costfunc

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

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

	// Rule H1: a replica which is unassigned got 2 point
	// Rule H2: a replicas which is assigned to a "daining" worker got 1 point
	// Rule H3: replicas from same shard should be on different workers (10 points penaty = illegal)
	{
		hard := int32(0)
		snap.AllShards.VisitAll(func(shardId data.ShardId, shard *ShardSnap) { // each shard
			dict := make(map[data.WorkerFullId]common.Unit)  // for H3
			for replicaId, replica := range shard.Replicas { // each replica
				if replica.LameDuck {
					continue
				}
				// this replica is assigned?
				replicaHasAssignment := false
				replicaHasHealthyAssignment := false // this replica is assigned to a healthy (not daining) worker
				for assignmentId := range replica.Assignments {
					replicaHasAssignment = true
					assignment, ok := snap.AllAssignments.Get(assignmentId)
					if !ok {
						klogging.Fatal(context.Background()).With("shard", shardId).With("replica", replicaId).With("assignId", assignmentId).Log("CostFuncSimpleProvider", "assignment not found")
						continue
					}
					if _, ok := dict[assignment.WorkerFullId]; ok {
						hard += 10 // illegal (H3)
					} else {
						dict[assignment.WorkerFullId] = common.Unit{}
						worker, ok := snap.AllWorkers.Get(assignment.WorkerFullId)
						if !ok {
							klogging.Fatal(context.Background()).With("shard", shardId).With("replica", replicaId).With("assignId", assignmentId).Log("CostFuncSimpleProvider", "worker not found")
							continue
						}
						if !worker.Draining {
							replicaHasHealthyAssignment = true
						}
					}
				}
				if !replicaHasAssignment {
					hard += 2 // H1
				} else if !replicaHasHealthyAssignment {
					hard += 1 // H2
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
			workerLoad := float64(len(worker.Assignments)) / float64(simple.CostfuncCfg.ShardCountCostNorm)
			soft += workerLoad * workerLoad
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
