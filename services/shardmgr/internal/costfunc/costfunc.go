package costfunc

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

type CostFuncProvider interface {
	CalCost(ctx context.Context, snap *Snapshot) Cost
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

type ReplicaView struct {
	ShardId     data.ShardId
	ReplicaIdx  data.ReplicaIdx
	LameDuck    bool
	Assignments map[data.AssignmentId]*AssignmentView
}

func (shardSnap *ShardSnap) CollectReplicas(ctx context.Context, snapshot *Snapshot) map[data.ReplicaIdx]*ReplicaView {
	replicaViews := make(map[data.ReplicaIdx]*ReplicaView)
	for replicaIdx, replicaSnap := range shardSnap.Replicas {
		replicaView := &ReplicaView{
			ShardId:     replicaSnap.ShardId,
			ReplicaIdx:  replicaSnap.ReplicaIdx,
			LameDuck:    replicaSnap.LameDuck,
			Assignments: snapshot.CollectAssignments(ctx, replicaSnap.Assignments),
		}
		replicaViews[replicaIdx] = replicaView
	}
	return replicaViews
}

func CountReplicas(replicaSnaps map[data.ReplicaIdx]*ReplicaView, fn func(*ReplicaView) bool) int {
	count := 0
	for _, rv := range replicaSnaps {
		if fn(rv) {
			count++
		}
	}
	return count
}

type AssignmentView struct {
	AssignmentId data.AssignmentId
	ShardId      data.ShardId
	ReplicaIdx   data.ReplicaIdx
	WorkerSnap   *WorkerSnap
}

func (snap *Snapshot) CollectAssignments(ctx context.Context, assigns map[data.AssignmentId]common.Unit) map[data.AssignmentId]*AssignmentView {
	assignmentViews := make(map[data.AssignmentId]*AssignmentView)
	for assignmentId := range assigns {
		assignment, ok := snap.AllAssignments.Get(assignmentId)
		if !ok {
			klogging.Fatal(ctx).With("assignId", assignmentId).With("snapshotId", snap.SnapshotId).Log("CostFuncSimpleProvider", "assignment not found")
			continue
		}
		worker, ok := snap.AllWorkers.Get(assignment.WorkerFullId)
		if !ok {
			klogging.Fatal(ctx).With("assignId", assignmentId).With("workerId", assignment.WorkerFullId.String()).With("snapshotId", snap.SnapshotId).Log("CostFuncSimpleProvider", "worker not found")
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
func (simple *CostFuncSimpleProvider) CalCost(ctx context.Context, snap *Snapshot) Cost {
	{
		// validate snapshot (replicas should be lame duck if they have no assignments)
		snap.AllShards.VisitAll(func(shardId data.ShardId, shard *ShardSnap) {
			for replicaIdx, replica := range shard.Replicas {
				if !replica.LameDuck && len(replica.Assignments) == 0 {
					klogging.Fatal(context.Background()).With("shardId", shardId).With("replicaIdx", replicaIdx).Log("CostFuncSimpleProvider", "replica without assignments should be lame duck")
				}
			}
		})
	}
	{
		// validate snapshot (assignment should be in the worker's assignments)
		snap.AllAssignments.VisitAll(func(assignmentId data.AssignmentId, assignment *AssignmentSnap) {
			worker, ok := snap.AllWorkers.Get(assignment.WorkerFullId)
			if !ok {
				klogging.Fatal(context.Background()).With("assignmentId", assignmentId).With("workerId", assignment.WorkerFullId.String()).Log("CostFuncSimpleProvider", "worker not found for assignment")
			}
			if _, ok := worker.Assignments[assignment.ShardId]; !ok {
				klogging.Fatal(context.Background()).With("assignmentId", assignmentId).With("workerId", assignment.WorkerFullId.String()).Log("CostFuncSimpleProvider", "assignment not found in worker's assignments")
			}
		})
	}
	totalCost := Cost{HardScore: 0, SoftScore: 0}

	// Rule H1: a replica which is unassigned got 2 point <deprecated by H6>
	// Rule H2: a replicas which is assigned to a "draining" worker got 1 point
	// Rule H3: replicas from same shard should be on different workers (10 points penaty = illegal)
	// Rule H4: a worker which has more than WorkerMaxAssignments got 10 points
	// Rule H5: for lame duck replica, each assignment got 1 point <deprecated by H7>
	// Rule H6: replicaCount < shard.TargetReplicaCount, each replica got 2 point
	// Rule H7: replicaCount > shard.TargetReplicaCount, each replica got 1 point
	{
		hard := int32(0)
		snap.AllShards.VisitAll(func(shardId data.ShardId, shard *ShardSnap) {
			// each shard
			cost := shard.GetShardCost(ctx, func() *Cost {
				soft := float64(0)
				hard := int32(0)
				replicas := shard.CollectReplicas(ctx, snap)
				currentReplicaCount := CountReplicas(replicas, func(rv *ReplicaView) bool { return !rv.LameDuck && len(rv.Assignments) > 0 })
				if currentReplicaCount < int(shard.TargetReplicaCount) {
					hard += int32(shard.TargetReplicaCount-currentReplicaCount) * 2 // H6
				} else if currentReplicaCount > int(shard.TargetReplicaCount) {
					hard += int32(currentReplicaCount - shard.TargetReplicaCount) // H7
				}
				dict := make(map[data.WorkerFullId]common.Unit) // for H3
				for _, replicaView := range replicas {
					// each replica
					// this replica is assigned?
					hasAssignmentNow := len(replicaView.Assignments) > 0
					hasAssignmentInFuture := CountAssignments(replicaView.Assignments, func(av *AssignmentView) bool { return !av.WorkerSnap.Draining }) > 0

					for _, av := range replicaView.Assignments {
						if _, ok := dict[av.WorkerSnap.WorkerFullId]; ok {
							hard += 10 // illegal (H3)
						} else {
							dict[av.WorkerSnap.WorkerFullId] = common.Unit{}
						}
					}
					if replicaView.LameDuck {
						hard += int32(len(replicaView.Assignments)) // H5
						continue
					}
					if !hasAssignmentNow {
						hard += 2 // H1
					} else if !hasAssignmentInFuture {
						hard += 1 // H2
					}
				}
				return &Cost{HardScore: hard, SoftScore: soft}
			})
			hard += cost.HardScore
		})
		// for _, shard := range snap.AllShards {
		// 	for _, replica := range shard.Replicas {
		// 		if len(replica.Assignments) == 0 {
		// 			hard++
		// 		}
		// 	}
		// }
		totalCost.HardScore += hard
	}

	// Part 2: Soft score
	// Rule H4: a worker which has more than WorkerMaxAssignments got 10 points
	{
		// soft := float64(0)
		// hard := int32(0)
		snap.AllWorkers.VisitAll(func(workerId data.WorkerFullId, worker *WorkerSnap) {
			cost := worker.GetWorkerCost(ctx, func() *Cost {
				soft := float64(0)
				hard := int32(0)
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
				return &Cost{HardScore: hard, SoftScore: soft}
			})
			totalCost.SoftScore += cost.SoftScore
			totalCost.HardScore += cost.HardScore
		})
	}

	totalCost.SoftScore *= 5000 // this makes soft score more readable
	return totalCost
}
