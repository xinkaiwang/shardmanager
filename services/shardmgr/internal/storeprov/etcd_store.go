package storeprov

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/krunloop"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/etcdprov"
)

type EtcdStore interface {
	Put(ctx context.Context, key string, value string)
}

type KvItem struct {
	Key   string
	Value string
}

// BufferedEtcdStore implements EtcdStore and buffers writes to etcd
type BufferedEtcdStore struct {
	etcd    etcdprov.EtcdProvider
	runloop *krunloop.RunLoop[*BufferedEtcdStore]
}

func NewBufferedEtcdStore(ctx context.Context) *BufferedEtcdStore {
	store := &BufferedEtcdStore{
		etcd: etcdprov.GetCurrentEtcdProvider(ctx),
	}
	store.runloop = krunloop.NewRunLoop(ctx, store, "etcdstore")
	go store.runloop.Run(ctx)
	return store
}

func (store *BufferedEtcdStore) Put(ctx context.Context, key string, value string) {
	eve := NewWriteEvent(key, value)
	store.runloop.EnqueueEvent(eve)
}

// BufferedEtcdStore implements CriticalResource
func (store *BufferedEtcdStore) IsResource() {}

// WriteEvent implements IEvent[*BufferedEtcdStore]
type WriteEvent struct {
	Key   string
	Value string
}

func NewWriteEvent(key string, value string) *WriteEvent {
	return &WriteEvent{
		Key:   key,
		Value: value,
	}
}

func (eve *WriteEvent) GetName() string {
	return "WriteEvent"
}

func (eve *WriteEvent) Process(ctx context.Context, resource *BufferedEtcdStore) {
	resource.etcd.Set(ctx, eve.Key, eve.Value)
}
