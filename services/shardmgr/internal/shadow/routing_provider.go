package shadow

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/unicorn/unicornjson"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

type RoutingProvider interface {
	SetInitialRouting(ctx context.Context, workerFullId data.WorkerFullId, routingEntry *unicornjson.WorkerEntryJson)
	StoreRoutingEntry(ctx context.Context, workerFullId data.WorkerFullId, routingEntry *unicornjson.WorkerEntryJson)
}

// defaultRoutingProvider implements RoutingProvider
type defaultRoutingProvider struct {
	pathManager *config.PathManager
	dict        map[data.WorkerFullId]*unicornjson.WorkerEntryJson
}

func NewDefaultRoutingProvider(pathManager *config.PathManager) RoutingProvider {
	return &defaultRoutingProvider{
		pathManager: pathManager,
		dict:        make(map[data.WorkerFullId]*unicornjson.WorkerEntryJson),
	}
}

func (provider *defaultRoutingProvider) SetInitialRouting(ctx context.Context, workerFullId data.WorkerFullId, routingEntry *unicornjson.WorkerEntryJson) {
	provider.dict[workerFullId] = routingEntry
}

func (provider *defaultRoutingProvider) StoreRoutingEntry(ctx context.Context, workerFullId data.WorkerFullId, routingEntry *unicornjson.WorkerEntryJson) {
	if routingEntry == nil {
		provider.DeleteRoutingEntry(ctx, workerFullId)
		return
	}
	existingEntry, ok := provider.dict[workerFullId]
	if ok {
		if existingEntry.EqualsTo(routingEntry) {
			// klogging.Info(ctx).
			// 	With("workerFullId", workerFullId).
			// 	With("reason", routingEntry.LastUpdateReason).
			// 	Log("StoreRoutingEntry", "routing entry is the same, skip")
			return
		}
	}
	path := provider.pathManager.FmtRoutingPath(workerFullId)
	GetCurrentEtcdStore(ctx).Put(ctx, path, routingEntry.ToJson(), "RoutingEntry")
	// klogging.Info(ctx).
	// 	With("workerFullId", workerFullId).
	// 	With("routingEntry", routingEntry.ToJson()).
	// 	With("reason", routingEntry.LastUpdateReason).
	// 	Log("StoreRoutingEntry", "routing entry written")
}

func (provider *defaultRoutingProvider) DeleteRoutingEntry(ctx context.Context, workerFullId data.WorkerFullId) {
	delete(provider.dict, workerFullId)
	path := provider.pathManager.FmtRoutingPath(workerFullId)
	GetCurrentEtcdStore(ctx).Put(ctx, path, "", "RoutingEntry")
	// klogging.Info(ctx).
	// 	With("workerFullId", workerFullId).
	// 	Log("StoreRoutingEntry", "routing entry deleted")
}
