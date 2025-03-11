package shadow

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"

	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kmetrics"
	"github.com/xinkaiwang/shardmanager/libs/xklib/krunloop"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/etcdprov"
)

var (
	// 使用互斥锁保护 currentEtcdStore
	currentEtcdStore EtcdStore
	storeMutex       sync.RWMutex

	// 跟踪EtcdStore的使用情况
	storeCreationCount    int
	storeAccessCount      int
	storeLastAccessCaller string
)

// DumpStoreStats 返回当前EtcdStore的使用统计信息
func DumpStoreStats() string {
	storeMutex.RLock()
	defer storeMutex.RUnlock()

	var storeType string
	if currentEtcdStore != nil {
		storeType = fmt.Sprintf("%T", currentEtcdStore)
	} else {
		storeType = "nil"
	}
	return fmt.Sprintf("EtcdStore类型=%s, 创建次数=%d, 访问次数=%d, 最后访问=%s",
		storeType, storeCreationCount, storeAccessCount, storeLastAccessCaller)
}

// GetCurrentEtcdStore 获取当前的EtcdStore，如果不存在则创建一个新的
func GetCurrentEtcdStore(ctx context.Context) EtcdStore {
	storeAccessCount++

	// 记录调用者信息（仅用于调试）
	_, file, line, ok := runtime.Caller(1)
	if ok {
		// 提取文件名（不含路径）
		if idx := strings.LastIndex(file, "/"); idx >= 0 {
			file = file[idx+1:]
		}
		storeLastAccessCaller = fmt.Sprintf("%s:%d", file, line)
	}

	// 检查当前存储
	storeMutex.RLock()
	store := currentEtcdStore
	storeMutex.RUnlock()

	if store != nil {
		return store
	}

	// 如果存储不存在，创建一个新的
	storeMutex.Lock()
	defer storeMutex.Unlock()

	// 双重检查锁定模式，确保不会创建多个实例
	if currentEtcdStore != nil {
		return currentEtcdStore
	}

	// 仅使用Info级别记录关键操作，避免过多日志
	storeCreationCount++
	klogging.Info(ctx).Log("event", "GetCurrentEtcdStore")
	store = NewBufferedEtcdStore(ctx)
	currentEtcdStore = store
	return store
}

// SetCurrentEtcdStore 直接设置当前的EtcdStore，主要用于测试
func SetCurrentEtcdStore(store EtcdStore) {
	storeMutex.Lock()
	defer storeMutex.Unlock()
	currentEtcdStore = store
}

func RunWithEtcdStore(store EtcdStore, fn func()) {
	ctx := context.Background()

	// 保存当前存储
	storeMutex.Lock()
	oldStore := currentEtcdStore
	currentEtcdStore = store
	storeMutex.Unlock()

	klogging.Info(ctx).
		With("oldStore", fmt.Sprintf("%T", oldStore)).
		With("newStore", fmt.Sprintf("%T", store)).
		Log("RunWithEtcdStore", "临时替换EtcdStore")

	defer func() {
		klogging.Info(ctx).
			With("restoredStore", fmt.Sprintf("%T", oldStore)).
			Log("RunWithEtcdStore", "恢复原始EtcdStore")

		// 恢复原始存储
		storeMutex.Lock()
		currentEtcdStore = oldStore
		storeMutex.Unlock()
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
	klogging.Info(ctx).Log("NewBufferedEtcdStore", "创建新的BufferedEtcdStore")
	etcdProvider := etcdprov.GetCurrentEtcdProvider(ctx)
	klogging.Info(ctx).With("providerType", fmt.Sprintf("%T", etcdProvider)).Log("NewBufferedEtcdStore", "使用的EtcdProvider类型")

	store := &BufferedEtcdStore{
		etcd: etcdProvider,
	}
	klogging.Info(ctx).Log("NewBufferedEtcdStore", "创建runloop")
	store.runloop = krunloop.NewRunLoop(ctx, store, "etcdstore")
	go store.runloop.Run(ctx)
	klogging.Info(ctx).Log("NewBufferedEtcdStore", "BufferedEtcdStore创建完成")
	return store
}

func (store *BufferedEtcdStore) Put(ctx context.Context, key string, value string, name string) {
	klogging.Info(ctx).With("key", key).With("valueLength", len(value)).With("name", name).Log("BufferedEtcdStorePut", "将写入请求加入队列")
	eve := NewWriteEvent(key, value, name)
	store.runloop.PostEvent(eve)
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

// ResetEtcdStore 重置全局EtcdStore变量，用于测试
func ResetEtcdStore() {
	storeMutex.Lock()
	defer storeMutex.Unlock()

	currentEtcdStore = nil
	storeCreationCount = 0
	storeAccessCount = 0
	storeLastAccessCaller = ""
}
