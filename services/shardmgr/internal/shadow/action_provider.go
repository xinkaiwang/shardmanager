package shadow

import (
	"context"

	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

type ActionProvider interface {
	StoreActionNode(ctx context.Context, proposalId data.ProposalId, moveNode *smgjson.MoveStateJson)
}

// DefaultActionProvider implements ActionProvider
type DefaultActionProvider struct {
	pathManager *config.PathManager
}

func NewDefaultActionProvider(pm *config.PathManager) *DefaultActionProvider {
	return &DefaultActionProvider{
		pathManager: pm,
	}
}

func (ap *DefaultActionProvider) StoreActionNode(ctx context.Context, proposalId data.ProposalId, moveNode *smgjson.MoveStateJson) {
	path := ap.pathManager.FmtMoveStatePath(proposalId)
	if moveNode == nil {
		GetCurrentEtcdStore(ctx).Put(ctx, path, "", "MoveState")
		return
	}
	GetCurrentEtcdStore(ctx).Put(ctx, path, moveNode.ToJson(), "MoveState")
}
