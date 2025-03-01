package shadow

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kmetrics"
	"github.com/xinkaiwang/shardmanager/libs/xklib/krunloop"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/etcdprov"
)

var (
	currentEtcdStore EtcdStore
)

func GetCurrentEtcdStore(ctx context.Context) EtcdStore {
	if currentEtcdStore == nil {
		currentEtcdStore = NewBufferedEtcdStore(ctx)
	}
	return currentEtcdStore
}

func RunWithEtcdStore(store EtcdStore, fn func()) {
	oldStore := currentEtcdStore
	currentEtcdStore = store
	defer func() {
		currentEtcdStore = oldStore
	}()
	fn()
}

type EtcdStore interface {
	// Put: put key-value pair to etcd. name is used for logging/metrics purposes only
	Put(ctx context.Context, key string, value string, name string)
}

type KvItem struct {
	Key   string
	Value string
	Name  string // for logging/metrics purposes only
}

var (
	EtcdStoreWriteSizeMetrics = kmetrics.CreateKmetric(context.Background(), "etcd_write_size", "desc", []string{"name"})
)

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

func (store *BufferedEtcdStore) Put(ctx context.Context, key string, value string, name string) {
	eve := NewWriteEvent(key, value, name)
	store.runloop.EnqueueEvent(eve)
}

// BufferedEtcdStore implements CriticalResource
func (store *BufferedEtcdStore) IsResource() {}

// WriteEvent implements IEvent[*BufferedEtcdStore]
type WriteEvent struct {
	Key   string
	Value string
	Name  string // for logging/metrics purposes only
}

func NewWriteEvent(key string, value string, name string) *WriteEvent {
	return &WriteEvent{
		Key:   key,
		Value: value,
		Name:  name,
	}
}

func (eve *WriteEvent) GetName() string {
	return eve.Name
}

func (eve *WriteEvent) Process(ctx context.Context, resource *BufferedEtcdStore) {
	size := len(eve.Value) + len(eve.Key)
	EtcdStoreWriteSizeMetrics.GetTimeSequence(ctx, eve.Name).Add(int64(size))
	if eve.Value == "" {
		resource.etcd.Delete(ctx, eve.Key)
		return
	}
	resource.etcd.Set(ctx, eve.Key, eve.Value)
}
