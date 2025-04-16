package solver

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kmetrics"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
)

var (
	SolverElapsedMsMetrics = kmetrics.CreateKmetric(context.Background(), "solver_elapsed_ms", "desc", []string{"name"})
)

type Solver interface {
	GetType() SolverType
	FindProposal(ctx context.Context, snapshot *costfunc.Snapshot) *costfunc.Proposal
}

type SolverType string

const (
	ST_SoftSolver     SolverType = "soft"
	ST_AssignSolver   SolverType = "assign"
	ST_UnassignSolver SolverType = "unassign"
)
