package solver

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

// UnassignSolver: implement Solver interface
type UnassignSolver struct {
}

func NewUnassignSolver() *UnassignSolver {
	return &UnassignSolver{}
}

func (us *UnassignSolver) GetType() SolverType {
	return ST_UnassignSolver
}

func (as *UnassignSolver) FindProposal(ctx context.Context, snapshot *costfunc.Snapshot) *costfunc.Proposal {
	// step 1: get the cost of the current snapshot
	baseCost := snapshot.GetCost()
	cfg := GetCurrentSolverConfigProvider().GetUnassignSolverConfig()

	var bestMove *costfunc.UnassignMove
	bestCost := baseCost

	stop := false
	for i := 0; i < cfg.ExplorePerRun && !stop; i++ {
		ke := kcommon.TryCatchRun(ctx, func() {
			var assign *costfunc.AssignmentSnap
			{
				// step 2: which assign?
				// candidate assign list
				candidateAssignment := []*costfunc.AssignmentSnap{}
				snapshot.AllAssignments.VisitAll(func(assignmentId data.AssignmentId, asgnSnap *costfunc.AssignmentSnap) {
					candidateAssignment = append(candidateAssignment, asgnSnap)
				})
				if len(candidateAssignment) == 0 {
					return
				}
				rnd := kcommon.RandomInt(ctx, len(candidateAssignment))
				assign = candidateAssignment[rnd]
			}
			newSnap := snapshot.Clone()
			newSnap.Unassign(assign.WorkerFullId, assign.ShardId, assign.ReplicaIdx, assign.AssignmentId, costfunc.AM_Strict, true)
			newCost := newSnap.GetCost()
			if newCost.IsLowerThan(bestCost) {
				bestCost = newCost
				bestMove = &costfunc.UnassignMove{
					Worker:       assign.WorkerFullId,
					Replica:      assign.GetReplicaFullId(),
					AssignmentId: assign.AssignmentId,
				}
			}
		})
		if ke != nil {
			klogging.Warning(ctx).WithError(ke).Log("UnsssignSolverTryCatch", "error in unassign solver loop, ignore this iteration")
		}
	}
	if bestMove == nil {
		return nil
	}

	proposal := costfunc.NewProposal(ctx, "UnassignSolver", baseCost.Substract(bestCost), snapshot.SnapshotId)
	proposal.StartTimeMs = kcommon.GetWallTimeMs()
	proposal.Move = bestMove
	proposal.Signature = proposal.GetSignature()
	proposal.OnClose = func(reason common.EnqueueResult) {
		elapsedMs := kcommon.GetWallTimeMs() - proposal.StartTimeMs
		klogging.Debug(ctx).With("reason", reason).With("elapsedMs", elapsedMs).With("solver", "UnassignSolver").Log("ProposalClosed", "")
	}
	return proposal
}
