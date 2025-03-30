package assemble

import (
	"context"

	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/core"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/solver"
)

func AssembleServiceState(ctx context.Context) *core.ServiceState {
	// ss
	ss := core.NewServiceState(ctx, "shardmgr")
	// solverGroup
	sg := solver.NewSolverGroup(ctx, ss.SnapshotFuture, ss.ProposalQueue.EnqueueProposal)
	sg.AddSolver(ctx, solver.NewSoftSolver())
	sg.AddSolver(ctx, solver.NewAssignSolver())
	sg.AddSolver(ctx, solver.NewUnassignSolver())
	ss.SolverGroup = sg
	// ShaddowState

	return ss
}
