package costfunc

import (
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

type CostFunc func(*Snapshot) Cost

func CostFuncSimple(snap *Snapshot) Cost {
	cost := Cost{HardScore: 0, SoftScore: 0}

	// Part 1: Hard score
	// all those replicas that are not assigned to any worker got 1 point (penalty)
	{
		hard := int32(0)
		allReplicas := map[data.ReplicaFullId]common.Unit{}
		for _, shard := range snap.AllShards {
			for _, replica := range shard.Replicas {
				allReplicas[replica.GetReplicaFullId()] = common.Unit{}
			}
		}
		for _, worker := range snap.AllWorkers {
			for _, assignment := range worker.Assignments {
				delete(allReplicas, assignment.GetReplicaFullId())
			}
		}
		hard += int32(len(allReplicas))
		cost.HardScore += hard
	}

	// Part 2: Soft score
	{
		soft := float64(0)
		hard := int32(0)
		for _, worker := range snap.AllWorkers {
			soft += float64(len(worker.Assignments)) / float64(snap.CostfuncCfg.ShardCountCostNorm)
			if len(worker.Assignments) > snap.CostfuncCfg.WorkerMaxAssignments {
				hard++
			}
		}
		cost.SoftScore += soft
		cost.HardScore += hard
	}

	cost.SoftScore *= 5000 // this makes soft score more readable
	return cost
}
