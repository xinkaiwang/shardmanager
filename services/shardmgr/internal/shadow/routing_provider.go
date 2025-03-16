package shadow

import (
	"context"
	"sync"

	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/unicorn/unicornjson"
)

var (
	currentCoutingProvider RoutingProvider
	routingProviderMutex   sync.RWMutex
)

func GetCurrentRoutingProvider() RoutingProvider {
	routingProviderMutex.Lock()
	defer routingProviderMutex.Unlock()

	if currentCoutingProvider == nil {
		currentCoutingProvider = NewDefaultRoutingProvider()
	}

	return currentCoutingProvider
}

type RoutingProvider interface {
	SetInitialRouting(ctx context.Context, workerFullId data.WorkerFullId, routingEntry *unicornjson.WorkerEntryJson)
	StoreRoutingEntry(ctx context.Context, workerFullId data.WorkerFullId, routingEntry *unicornjson.WorkerEntryJson)
}

// defaultRoutingProvider implements RoutingProvider
type defaultRoutingProvider struct {
	dict map[data.WorkerFullId]*unicornjson.WorkerEntryJson
}

func NewDefaultRoutingProvider() RoutingProvider {
	return &defaultRoutingProvider{
		dict: make(map[data.WorkerFullId]*unicornjson.WorkerEntryJson),
	}
}

func (provider *defaultRoutingProvider) SetInitialRouting(ctx context.Context, workerFullId data.WorkerFullId, routingEntry *unicornjson.WorkerEntryJson) {
	provider.dict[workerFullId] = routingEntry
}

func (provider *defaultRoutingProvider) StoreRoutingEntry(ctx context.Context, workerFullId data.WorkerFullId, routingEntry *unicornjson.WorkerEntryJson) {
	existingEntry, ok := provider.dict[workerFullId]
	if ok {
		if existingEntry.EqualsTo(routingEntry) {
			klogging.Info(ctx).
				With("workerFullId", workerFullId).
				With("reason", routingEntry.LastUpdateReason).
				Log("StoreRoutingEntry", "routing entry is the same, skip")
			return
		}
	}
	path := config.GetCurrentPathManager().FmtRoutingPath(workerFullId)
	GetCurrentEtcdStore(ctx).Put(ctx, path, routingEntry.ToJson(), "RoutingEntry")
	klogging.Info(ctx).
		With("workerFullId", workerFullId).
		With("routingEntry", routingEntry.ToJson()).
		With("reason", routingEntry.LastUpdateReason).
		Log("StoreRoutingEntry", "routing entry written")
}
