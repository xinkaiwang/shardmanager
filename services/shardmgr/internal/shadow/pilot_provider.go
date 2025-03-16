package shadow

import (
	"context"
	"sync"

	"github.com/xinkaiwang/shardmanager/services/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

var (
	// 使用互斥锁保护 currentPilotProvider
	currentPilotProvider PilotProvider
	pilotStoreMutex      sync.RWMutex
)

// GetCurrentPilotProvider 获取当前的PilotStore，如果不存在则创建一个新的
func GetCurrentPilotProvider() PilotProvider {
	// 使用互斥锁保护全局变量的读写
	pilotStoreMutex.Lock()
	defer pilotStoreMutex.Unlock()

	if currentPilotProvider == nil {
		currentPilotProvider = newPilotStore()
	}

	return currentPilotProvider
}

type PilotProvider interface {
	StorePilotNode(ctx context.Context, workerFullId data.WorkerFullId, pilotNode *cougarjson.PilotNodeJson)
}

func newPilotStore() PilotProvider {
	return &defaultPilotStore{}
}

type defaultPilotStore struct {
}

func (store *defaultPilotStore) StorePilotNode(ctx context.Context, workerFullId data.WorkerFullId, pilotNode *cougarjson.PilotNodeJson) {
	path := config.GetCurrentPathManager().FmtPilotPath(workerFullId)
	GetCurrentEtcdStore(ctx).Put(ctx, path, pilotNode.ToJson(), "PilotNode")
}
