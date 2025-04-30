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

type AssignmentView struct {
	AssignmentId data.AssignmentId
	ShardId      data.ShardId
	ReplicaIdx   data.ReplicaIdx
	WorkerSnap   *WorkerSnap
}

func (snap *Snapshot) CollectAssignments(assigns map[data.AssignmentId]common.Unit) map[data.AssignmentId]*AssignmentView {
	assignmentViews := make(map[data.AssignmentId]*AssignmentView)
	for assignmentId := range assigns {
		assignment, ok := snap.AllAssignments.Get(assignmentId)
		if !ok {
			klogging.Fatal(context.Background()).With("assignId", assignmentId).Log("CostFuncSimpleProvider", "assignment not found")
			continue
		}
		worker, ok := snap.AllWorkers.Get(assignment.WorkerFullId)
		if !ok {
			klogging.Fatal(context.Background()).With("assignId", assignmentId).Log("CostFuncSimpleProvider", "worker not found")
			continue
		}
		av := &AssignmentView{
			AssignmentId: assignment.AssignmentId,
			ShardId:      assignment.ShardId,
			ReplicaIdx:   assignment.ReplicaIdx,
			WorkerSnap:   worker,
		}
		assignmentViews[assignmentId] = av
	}
	return assignmentViews
}

func CountAssignments(assigns map[data.AssignmentId]*AssignmentView, fn func(*AssignmentView) bool) int {
	count := 0
	for _, av := range assigns {
		if fn(av) {
			count++
		}
	}
	return count
}

// CalCost implements the CostFuncProvider interface
func (simple *CostFuncSimpleProvider) CalCost(snap *Snapshot) Cost {
	cost := Cost{HardScore: 0, SoftScore: 0}

	// Rule H1: a replica which is unassigned got 2 point
	// Rule H2: a replicas which is assigned to a "draining" worker got 1 point
	// Rule H3: replicas from same shard should be on different workers (10 points penaty = illegal)
	// Rule H4: a worker which has more than WorkerMaxAssignments got 10 points
	// Rule H5: for lame duck replica, each assignment got 1 point
	{
		hard := int32(0)
		snap.AllShards.VisitAll(func(shardId data.ShardId, shard *ShardSnap) {
			// each shard
			dict := make(map[data.WorkerFullId]common.Unit) // for H3
			for _, replica := range shard.Replicas {
				// each replica
				// this replica is assigned?
				assignments := snap.CollectAssignments(replica.Assignments)
				hasAssignmentNow := len(assignments) > 0
				hasAssignmentInFuture := CountAssignments(assignments, func(av *AssignmentView) bool { return !av.WorkerSnap.Draining }) > 0

				for _, av := range assignments {
					if _, ok := dict[av.WorkerSnap.WorkerFullId]; ok {
						hard += 10 // illegal (H3)
					} else {
						dict[av.WorkerSnap.WorkerFullId] = common.Unit{}
					}
				}
				if replica.LameDuck {
					hard += int32(len(assignments)) // H5
					continue
				}
				if !hasAssignmentNow {
					hard += 2 // H1
				} else if !hasAssignmentInFuture {
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
	// Rule H4: a worker which has more than WorkerMaxAssignments got 10 points
	{
		soft := float64(0)
		hard := int32(0)
		snap.AllWorkers.VisitAll(func(workerId data.WorkerFullId, worker *WorkerSnap) {
			workerLoad := float64(len(worker.Assignments)) / float64(simple.CostfuncCfg.ShardCountCostNorm)
			soft += workerLoad * workerLoad
			if len(worker.Assignments) > int(simple.CostfuncCfg.WorkerMaxAssignments) {
				hard += 10 // illegal (H4)
			}
			dict := make(map[data.ShardId]common.Unit) // for H3
			for _, assignmentId := range worker.Assignments {
				assignment, ok := snap.AllAssignments.Get(assignmentId)
				if !ok {
					klogging.Fatal(context.Background()).With("worker", workerId).With("assignId", assignmentId).Log("CostFuncSimpleProvider", "assignment not found")
					continue
				}
				if _, ok := dict[assignment.ShardId]; ok {
					hard += 10 // illegal (H3)
				} else {
					dict[assignment.ShardId] = common.Unit{}
				}
			}
		})
		cost.SoftScore += soft
		cost.HardScore += hard
	}

	cost.SoftScore *= 5000 // this makes soft score more readable
	return cost
}
