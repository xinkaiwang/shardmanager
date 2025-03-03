package solver

import (
	"context"

	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
)

type Solver interface {
	FindProposal(ctx context.Context, snapshot *costfunc.Snapshot) *costfunc.Proposal
}
