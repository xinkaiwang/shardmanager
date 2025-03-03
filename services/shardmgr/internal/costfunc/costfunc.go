package costfunc

var (
	currentCostFuncProvider CostFuncProvider
)

func GetCurrentCostFuncProvider() CostFuncProvider {
	if currentCostFuncProvider == nil {
		currentCostFuncProvider = NewCostFuncSimpleProvider()
	}
	return currentCostFuncProvider
}

type CostFuncProvider interface {
	CalCost(snap *Snapshot) Cost
}

// CostFuncSimpleProvider implements the CostFuncProvider interface
type CostFuncSimpleProvider struct {
}

func NewCostFuncSimpleProvider() *CostFuncSimpleProvider {
	return &CostFuncSimpleProvider{}
}

// CalCost implements the CostFuncProvider interface
func (simple *CostFuncSimpleProvider) CalCost(snap *Snapshot) Cost {
	cost := Cost{HardScore: 0, SoftScore: 0}

	// Part 1: Hard score
	// all those replicas that are not assigned to any worker got 1 point (penalty)
	{
		hard := int32(0)
		// allReplicas := map[data.ReplicaFullId]common.Unit{}
		for _, shard := range snap.AllShards {
			for _, replica := range shard.Replicas {
				if len(replica.Assignments) == 0 {
					hard++
				}
			}
		}
		cost.HardScore += hard
	}

	// Part 2: Soft score
	{
		soft := float64(0)
		hard := int32(0)
		for _, worker := range snap.AllWorkers {
			soft += float64(len(worker.Assignments)) / float64(snap.CostfuncCfg.ShardCountCostNorm)
			if len(worker.Assignments) > int(snap.CostfuncCfg.WorkerMaxAssignments) {
				hard++
			}
		}
		cost.SoftScore += soft
		cost.HardScore += hard
	}

	cost.SoftScore *= 5000 // this makes soft score more readable
	return cost
}
