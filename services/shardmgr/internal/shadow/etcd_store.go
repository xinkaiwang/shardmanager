package shadow

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
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
	klogging.Info(ctx).With("key", key).With("valueLength", len(value)).With("name", name).Log("BufferedEtcdStorePut", "将写入请求加入队列")
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
	klogging.Info(ctx).With("key", eve.Key).With("valueLength", len(eve.Value)).With("name", eve.Name).Log("WriteEventProcess", "处理写入请求")
	if eve.Value == "" {
		klogging.Info(ctx).With("key", eve.Key).Log("WriteEventDelete", "从etcd中删除键")
		resource.etcd.Delete(ctx, eve.Key)
		return
	}
	klogging.Info(ctx).With("key", eve.Key).With("valueLength", len(eve.Value)).Log("WriteEventSet", "向etcd写入数据")
	resource.etcd.Set(ctx, eve.Key, eve.Value)
}
