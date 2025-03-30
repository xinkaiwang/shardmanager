package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/shadow"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/solver"
)

func AssembleSsAll(ctx context.Context, name string) *ServiceState { // name is for logging purpose only
	// ss
	ss := AssembleSsWithShadowState(ctx, name)

	// solverGroup
	sg := solver.NewSolverGroup(ctx, ss.SnapshotFuture, ss.ProposalQueue.EnqueueProposal)
	sg.AddSolver(ctx, solver.NewSoftSolver())
	sg.AddSolver(ctx, solver.NewAssignSolver())
	sg.AddSolver(ctx, solver.NewUnassignSolver())
	ss.SolverGroup = sg

	return ss
}

func AssembleSsWithShadowState(ctx context.Context, name string) *ServiceState { // name is for logging purpose only
	ss := NewServiceState(ctx, name)
	// shadow state
	shadowState := shadow.NewShadowState(ctx, ss.PathManager)
	ss.ShadowState = shadowState
	ss.storeProvider = shadowState

	// init
	ss.Init(ctx)
	// start runloop
	go ss.runloop.Run(ctx)
	return ss
}
