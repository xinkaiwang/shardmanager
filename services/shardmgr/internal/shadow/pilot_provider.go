package shadow

import (
	"context"
	"sync"

	"github.com/xinkaiwang/shardmanager/services/cougar/cougarjson"
)

var (
	// 使用互斥锁保护 currentPilotStore
	currentPilotStore PilotStore
	pilotStoreMutex   sync.RWMutex
)

// GetCurrentPilotStore 获取当前的PilotStore，如果不存在则创建一个新的
func GetCurrentPilotStore() PilotStore {
	// 使用互斥锁保护全局变量的读写
	pilotStoreMutex.Lock()
	defer pilotStoreMutex.Unlock()

	if currentPilotStore == nil {
		currentPilotStore = newPilotStore()
	}

	return currentPilotStore
}

type PilotStore interface {
	StorePilotNode(ctx context.Context, path string, pilotNode *cougarjson.PilotNodeJson)
}

func newPilotStore() PilotStore {
	return &defaultPilotStore{}
}

type defaultPilotStore struct {
}

func (store *defaultPilotStore) StorePilotNode(ctx context.Context, path string, pilotNode *cougarjson.PilotNodeJson) {
	GetCurrentEtcdStore(ctx).Put(ctx, path, pilotNode.ToJson(), "PilotNode")
}
