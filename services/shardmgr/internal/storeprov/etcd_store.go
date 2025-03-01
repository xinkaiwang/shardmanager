package storeprov

import (
	"context"
	"sync"

	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/etcdprov"
)

type EtcdStore interface {
	Put(ctx context.Context, key string, value string)
}

type KvItem struct {
	Key   string
	Value string
}

type BufferedEtcdStore struct {
	etcd   etcdprov.EtcdProvider
	buffer []KvItem
	mu     sync.Mutex
}

func NewBufferedEtcdStore(ctx context.Context) *BufferedEtcdStore {
	return &BufferedEtcdStore{
		etcd: etcdprov.GetCurrentEtcdProvider(ctx),
	}
}

func (s *BufferedEtcdStore) Put(ctx context.Context, key string, value string) {
	s.etcd.Set(ctx, key, value)
}
