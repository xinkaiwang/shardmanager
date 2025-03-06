package solver

import (
	"context"

	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
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
