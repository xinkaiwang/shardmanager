package shadow

import (
	"context"
	"sync"

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
	StoreRoutingEntry(ctx context.Context, workerFullId data.WorkerFullId, routingEntry *unicornjson.WorkerEntryJson)
}

// defaultRoutingProvider implements RoutingProvider
type defaultRoutingProvider struct {
}

func NewDefaultRoutingProvider() RoutingProvider {
	return &defaultRoutingProvider{}
}

func (provider *defaultRoutingProvider) StoreRoutingEntry(ctx context.Context, workerFullId data.WorkerFullId, routingEntry *unicornjson.WorkerEntryJson) {
	path := config.GetCurrentPathManager().FmtRoutingPath(workerFullId)
	GetCurrentEtcdStore(ctx).Put(ctx, path, routingEntry.ToJson(), "RoutingEntry")
}
