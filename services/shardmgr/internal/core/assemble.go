package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/shadow"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/solver"
)

func AssembleSsAll(ctx context.Context, name string) *ServiceState { // name is for logging purpose only
	// ss
	ss := NewServiceState(ctx, name)
	// shadow state
	shadowState := shadow.NewShadowState(ctx, ss.PathManager)
	ss.ShadowState = shadowState
	ss.storeProvider = shadowState

	// init
	ss.Init(klogging.EmbedTraceId(ctx, "it_"+kcommon.RandomString(ctx, 8))) // it_ = init trace

	// solverGroup
	sg := solver.NewSolverGroup(ctx, ss.GetSnapshotFutureForClone(), ss.ProposalQueue.Push)
	sg.AddSolver(ctx, solver.NewSoftSolver())
	sg.AddSolver(ctx, solver.NewAssignSolver())
	sg.AddSolver(ctx, solver.NewUnassignSolver())
	ss.SolverGroup = sg
	ss.SolverGroup.OnSnapshot(ctx, ss.GetSnapshotFutureForClone(), "AssembleSsAll.init")

	go ss.runloop.Run(ctx) // rl_ = runloop trace
	return ss
}

// for testing only
func AssembleSsWithShadowState(ctx context.Context, name string) *ServiceState { // name is for logging purpose only
	ss := NewServiceState(ctx, name)
	// shadow state
	shadowState := shadow.NewShadowState(ctx, ss.PathManager)
	ss.ShadowState = shadowState
	ss.storeProvider = shadowState

	// init
	ss.Init(klogging.EmbedTraceId(ctx, "it_"+kcommon.RandomString(ctx, 8))) // it_ = init trace
	// start runloop
	go ss.runloop.Run(klogging.EmbedTraceId(ctx, "rl_"+kcommon.RandomString(ctx, 6))) // rl_ = runloop trace
	return ss
}
