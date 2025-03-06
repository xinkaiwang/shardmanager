package solver

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

// SoftSolver implements the Solver interface.
type SoftSolver struct {
}

func NewSoftSolver() *SoftSolver {
	return &SoftSolver{}
}

func (ss *SoftSolver) FindProposal(ctx context.Context, snapshot *costfunc.Snapshot) *costfunc.Proposal {
	// step 1: get the cost of the current snapshot
	costProvider := costfunc.GetCurrentCostFuncProvider()
	baseCost := costProvider.CalCost(snapshot)
	softSolverCfg := GetCurrentSolverConfigProvider().GetSoftSolverConfig()

	var bestMove *costfunc.SimpleMove
	bestCost := baseCost
	// step 2: generate a proposal
	// explore random moves and see if it can reduce the cost
	stop := false
	for i := 0; i < softSolverCfg.ExplorePerRun && !stop; i++ {
		var srcWorkerId data.WorkerFullId
		{
			// Step 3: which source worker?
			// candidate worker list
			candidateWorkers := []data.WorkerFullId{}
			snapshot.AllWorkers.VisitAll(func(fullId data.WorkerFullId, worker *costfunc.WorkerSnap) {
				if len(worker.Assignments) > 0 {
					candidateWorkers = append(candidateWorkers, fullId)
				}
			})
			if len(candidateWorkers) == 0 {
				// no worker has any assignment
				stop = true
				continue
			}
			rnd := kcommon.RandomInt(ctx, len(candidateWorkers))
			srcWorkerId = candidateWorkers[rnd]
		}
		var assignment *costfunc.AssignmentSnap
		{
			// Step 4: which replica?
			srcWorker, _ := snapshot.AllWorkers.Get(srcWorkerId)
			// candidate replica list
			candidateReplicas := []*costfunc.AssignmentSnap{}
			for _, assignment := range srcWorker.Assignments {
				asgnSnap, _ := snapshot.AllAssigns.Get(assignment)
				candidateReplicas = append(candidateReplicas, asgnSnap)
			}
			if len(candidateReplicas) == 0 {
				// no replica is assigned to this worker
				continue
			}
			rnd := kcommon.RandomInt(ctx, len(candidateReplicas))
			assignment = candidateReplicas[rnd]
		}
		var destWorkerId data.WorkerFullId
		{
			// Step 5: which target worker?
			// candidate worker list
			candidateWorkers := []data.WorkerFullId{}
			snapshot.AllWorkers.VisitAll(func(fullId data.WorkerFullId, worker *costfunc.WorkerSnap) {
				if fullId != srcWorkerId && worker.CanAcceptAssignment(assignment.ShardId) {
					candidateWorkers = append(candidateWorkers, fullId)
				}
			})
			if len(candidateWorkers) == 0 {
				// no worker has can take this replica
				continue
			}
			rnd := kcommon.RandomInt(ctx, len(candidateWorkers))
			destWorkerId = candidateWorkers[rnd]
		}

		// Step 6: ok, now we have src/shard/dest, is this a good move?
		snapshotCopy := snapshot.Clone()
		move := &costfunc.SimpleMove{
			Replica:          assignment.GetReplicaFullId(),
			SrcAssignmentId:  assignment.AssignmentId,
			DestAssignmentId: data.AssignmentId(kcommon.RandomString(ctx, 8)),
			Src:              srcWorkerId,
			Dst:              destWorkerId,
		}
		move.Apply(snapshotCopy)
		newCost := costProvider.CalCost(snapshotCopy)
		if !newCost.IsLowerThan(bestCost) {
			continue
		}
		bestCost = newCost
		bestMove = move
	}
	if bestMove == nil {
		return nil
	}
	proposal := costfunc.NewProposal(ctx, "SoftSolver", baseCost.Substract(bestCost), snapshot.SnapshotId)
	proposal.Move = bestMove
	proposal.OnClose = func(reason common.EnqueueResult) {
		elapsedMs := kcommon.GetWallTimeMs() - proposal.StartTimeMs
		klogging.Debug(ctx).With("reason", reason).With("elapsedMs", elapsedMs).With("solver", "SoftSolver").Log("ProposalClosed", "")
	}
	return proposal
}
