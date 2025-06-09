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
	baseCost := snapshot.GetCost(ctx)
	asCfg := GetCurrentSolverConfigProvider().GetAssignSolverConfig()
	// klogging.Info(ctx).With("baseCost", baseCost).With("solver", "AssignSolver").Log("FindProposal", "start")

	var bestMove *costfunc.AssignMove
	bestCost := baseCost

	// candidate worker list
	candidateWorkers := []data.WorkerFullId{}
	snapshot.AllWorkers.VisitAll(func(fullId data.WorkerFullId, worker *costfunc.WorkerSnap) {
		if worker.NotTarget {
			// this worker is not target, skip it
			return
		}
		candidateWorkers = append(candidateWorkers, fullId)
	})
	if len(candidateWorkers) == 0 {
		// no worker can accept this assignment
		return nil
	}

	stop := false
	for i := 0; i < asCfg.ExplorePerRun && !stop; i++ {
		ke := kcommon.TryCatchRun(ctx, func() {
			var replicaCandidate data.ReplicaFullId
			{
				// step 2: which shard?
				// candidate replica list
				candidateReplicas := []data.ReplicaFullId{}
				snapshot.AllShards.VisitAll(func(shardId data.ShardId, shard *costfunc.ShardSnap) {
					replicaViews := shard.CollectReplicas(ctx, snapshot)
					if costfunc.CountReplicas(replicaViews, func(rv *costfunc.ReplicaView) bool {
						return !rv.LameDuck
					}) < shard.TargetReplicaCount {
						candidateReplicas = append(candidateReplicas, data.NewReplicaFullId(shardId, -1)) // replicaIdx will be set later, -1 means placeholder
					}
					for replicaIdx, replicaView := range replicaViews {
						if replicaView.LameDuck {
							continue
						}
						if len(replicaView.Assignments) > 0 {
							continue
						}
						candidateReplicas = append(candidateReplicas, data.NewReplicaFullId(shardId, replicaIdx))
					}
				})
				if len(candidateReplicas) == 0 {
					// no replica needs new assignment
					stop = true
					return
				}
				rnd := kcommon.RandomInt(ctx, len(candidateReplicas))
				replicaCandidate = candidateReplicas[rnd]
			}
			var destWorkerId data.WorkerFullId
			{
				// step 3: which dest worker?
				rnd := kcommon.RandomInt(ctx, len(candidateWorkers))
				destWorkerId = candidateWorkers[rnd]
			}
			{
				// step 3b: is this move valid?
				// check if the replica is already assigned to the dest worker
				workerSnap, _ := snapshot.AllWorkers.Get(destWorkerId)
				if !workerSnap.CanAcceptAssignment(replicaCandidate.ShardId) {
					return // not valid
				}
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
			move.Apply(newSnapshot, costfunc.AM_Strict)
			// calculate the cost of the new snapshot
			// newCost := costProvider.CalCost(newSnapshot)
			newCost := newSnapshot.GetCost(ctx)
			if newCost.IsLowerThan(bestCost) {
				bestCost = newCost
				bestMove = move
			}
		})
		if ke != nil {
			klogging.Warning(ctx).WithError(ke).Log("AssignSolverTryCatch", "error in assign solver loop, ignore this iteration")
		}
	}
	if bestMove == nil {
		return nil
	}

	// step 5: remember the replicaIdx is not set yet?
	if bestMove.Replica.ReplicaIdx == -1 {
		// find the next available replicaIdx
		shardSnap, _ := snapshot.AllShards.Get(bestMove.Replica.ShardId)
		// replicas := shardSnap.Replicas
		// firstAvailableIdx := 0
		// for replicas[data.ReplicaIdx(firstAvailableIdx)] != nil {
		// 	firstAvailableIdx++
		// }
		// bestMove.Replica.ReplicaIdx = data.ReplicaIdx(firstAvailableIdx)
		bestMove.Replica.ReplicaIdx = shardSnap.FindNextAvaReplicaIdx()
	}

	// step 6: create a proposal
	proposal := costfunc.NewProposal(ctx, "AssignSolver", baseCost.Substract(bestCost), snapshot.SnapshotId)
	proposal.Move = bestMove
	proposal.Signature = bestMove.GetSignature()
	proposal.OnClose = func(reason common.EnqueueResult) {
		elapsedMs := kcommon.GetWallTimeMs() - proposal.StartTimeMs
		klogging.Debug(ctx).With("reason", reason).With("elapsedMs", elapsedMs).With("solver", "AssignSolver").Log("ProposalClosed", "")
	}
	klogging.Info(ctx).With("proposalId", proposal.ProposalId).With("solver", "AssignSolver").With("signature", proposal.GetSignature()).Log("ProposalCreated", "")
	return proposal
}
