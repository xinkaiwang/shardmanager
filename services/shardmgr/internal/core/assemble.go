package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/solver"
)

func CreateServiceState(ctx context.Context) *ServiceState {
	ss := NewServiceState(ctx, "shardmgr")
	sg := solver.NewSolverGroup(ctx, ss.SnapshotFuture, ss.ProposalQueue.EnqueueProposal)
	sg.AddSolver(ctx, solver.NewSoftSolver())
	sg.AddSolver(ctx, solver.NewAssignSolver())
	sg.AddSolver(ctx, solver.NewUnassignSolver())
	ss.solverGroup = sg
	return ss
}
