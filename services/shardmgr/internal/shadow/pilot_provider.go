package shadow

import (
	"context"

	"github.com/xinkaiwang/shardmanager/services/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

type PilotProvider interface {
	SetInitialPilotNode(ctx context.Context, workerFullId data.WorkerFullId, pilotNode *cougarjson.PilotNodeJson)
	StorePilotNode(ctx context.Context, workerFullId data.WorkerFullId, pilotNode *cougarjson.PilotNodeJson)
}

func NewPilotStore(pathManager *config.PathManager) PilotProvider {
	return &defaultPilotStore{
		pathManager: pathManager,
		dict:        make(map[data.WorkerFullId]*cougarjson.PilotNodeJson),
	}
}

type defaultPilotStore struct {
	pathManager *config.PathManager
	dict        map[data.WorkerFullId]*cougarjson.PilotNodeJson
}

func (store *defaultPilotStore) SetInitialPilotNode(ctx context.Context, workerFullId data.WorkerFullId, pilotNode *cougarjson.PilotNodeJson) {
	store.dict[workerFullId] = pilotNode
}

func (store *defaultPilotStore) StorePilotNode(ctx context.Context, workerFullId data.WorkerFullId, pilotNode *cougarjson.PilotNodeJson) {
	if pilotNode == nil {
		store.DeleteNode(ctx, workerFullId)
		return
	}
	existingNode, ok := store.dict[workerFullId]
	if ok {
		if existingNode.EqualsTo(pilotNode) {
			// klogging.Info(ctx).
			// 	With("workerFullId", workerFullId).
			// 	With("reason", pilotNode.LastUpdateReason).
			// 	Log("StorePilotNode", "pilot node is the same, skip")
			return
		}
	}
	path := config.GetCurrentPathManager().FmtPilotPath(workerFullId)
	GetCurrentEtcdStore(ctx).Put(ctx, path, pilotNode.ToJson(), "PilotNode")
	// klogging.Info(ctx).
	// 	With("workerFullId", workerFullId).
	// 	With("pilotNode", pilotNode.ToJson()).
	// 	With("reason", pilotNode.LastUpdateReason).
	// 	Log("StorePilotNode", "pilot node stored")
}

func (store *defaultPilotStore) DeleteNode(ctx context.Context, workerFullId data.WorkerFullId) {
	delete(store.dict, workerFullId)
	path := config.GetCurrentPathManager().FmtPilotPath(workerFullId)
	GetCurrentEtcdStore(ctx).Put(ctx, path, "", "PilotNode")
	// klogging.Info(ctx).
	// 	With("workerFullId", workerFullId).
	// 	Log("StorePilotNode", "pilot node deleted")
}
