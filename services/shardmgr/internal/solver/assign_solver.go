package solver

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

type AssignSolver struct {
}

func NewAssignSolver() *AssignSolver {
	return &AssignSolver{}
}

func (as *AssignSolver) GetType() SolverType {
	return ST_AssignSolver
}
func (as *AssignSolver) FindProposal(ctx context.Context, snapshot *costfunc.Snapshot) *costfunc.Proposal {
	// step 1: get the cost of the current snapshot
	// costProvider := costfunc.GetCurrentCostFuncProvider()
	// baseCost := costProvider.CalCost(snapshot)
	baseCost := snapshot.GetCost()
	asCfg := GetCurrentSolverConfigProvider().GetAssignSolverConfig()

	var bestMove *costfunc.AssignMove
	bestCost := baseCost

	stop := false
	for i := 0; i < asCfg.ExplorePerRun && !stop; i++ {
		var replicaCandidate data.ReplicaFullId
		{
			// step 2: which shard?
			// candidate replica list
			candidateReplicas := []data.ReplicaFullId{}
			snapshot.AllShards.VisitAll(func(shardId data.ShardId, shard *costfunc.ShardSnap) {
				for _, replica := range shard.Replicas {
					if len(replica.Assignments) > 0 {
						continue
					}
					candidateReplicas = append(candidateReplicas, replica.GetReplicaFullId())
				}
			})
			if len(candidateReplicas) == 0 {
				// no replica needs new assignment
				return nil
			}
			rnd := kcommon.RandomInt(ctx, len(candidateReplicas))
			replicaCandidate = candidateReplicas[rnd]
		}
		var destWorkerId data.WorkerFullId
		{
			// step 3: which dest worker?
			// candidate worker list
			candidateWorkers := []data.WorkerFullId{}
			snapshot.AllWorkers.VisitAll(func(fullId data.WorkerFullId, worker *costfunc.WorkerSnap) {
				if worker.CanAcceptAssignment(replicaCandidate.ShardId) {
					candidateWorkers = append(candidateWorkers, fullId)
				}
			})
			if len(candidateWorkers) == 0 {
				// no worker can accept this assignment
				return nil
			}
			rnd := kcommon.RandomInt(ctx, len(candidateWorkers))
			destWorkerId = candidateWorkers[rnd]
		}
		// step 4: is this move better?
		// make a copy of the snapshot
		newSnapshot := snapshot.Clone()
		// remove the assignment from the source worker
		move := &costfunc.AssignMove{
			Replica:      replicaCandidate,
			AssignmentId: data.AssignmentId(kcommon.RandomString(ctx, 8)),
			Worker:       destWorkerId,
		}
		move.Apply(newSnapshot)
		// calculate the cost of the new snapshot
		// newCost := costProvider.CalCost(newSnapshot)
		newCost := newSnapshot.GetCost()
		if newCost.IsLowerThan(bestCost) {
			bestCost = newCost
			bestMove = move
		}
	}
	if bestMove == nil {
		return nil
	}
	proposal := costfunc.NewProposal(ctx, "AssignMove", baseCost.Substract(bestCost), snapshot.SnapshotId)
	proposal.Move = bestMove
	proposal.OnClose = func(reason common.EnqueueResult) {
		elapsedMs := kcommon.GetWallTimeMs() - proposal.StartTimeMs
		klogging.Debug(ctx).With("reason", reason).With("elapsedMs", elapsedMs).With("solver", "AssignSolver").Log("ProposalClosed", "")
	}
	return proposal
}
