package shadow

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"

	"log/slog"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
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
	// 使用互斥锁保护全局变量的读写
	storeMutex.Lock()
	storeAccessCount++
	storeMutex.Unlock()

	// 记录调用者信息（仅用于调试）
	_, file, line, ok := runtime.Caller(1)
	if ok {
		// 提取文件名（不含路径）
		if idx := strings.LastIndex(file, "/"); idx >= 0 {
			file = file[idx+1:]
		}

		// 使用互斥锁保护对 storeLastAccessCaller 的写入
		storeMutex.Lock()
		storeLastAccessCaller = fmt.Sprintf("%s:%d", file, line)
		storeMutex.Unlock()
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
	slog.InfoContext(ctx, "创建新的EtcdStore实例", slog.String("event", "EtcdStoreCreation"))
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

	slog.InfoContext(ctx, "临时替换EtcdStore", slog.String("event", "RunWithEtcdStore"), slog.String("oldStore", fmt.Sprintf("%T", oldStore)), slog.String("newStore", fmt.Sprintf("%T", store)))

	defer func() {
		slog.InfoContext(ctx, "恢复原始EtcdStore", slog.String("event", "RunWithEtcdStore"), slog.String("restoredStore", fmt.Sprintf("%T", oldStore)))

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
	Shutdown(ctx context.Context)
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
	slog.InfoContext(ctx, "创建新的BufferedEtcdStore", slog.String("event", "NewBufferedEtcdStore"))
	etcdProvider := etcdprov.GetCurrentEtcdProvider(ctx)
	slog.InfoContext(ctx, "使用的EtcdProvider类型", slog.String("event", "NewBufferedEtcdStore"), slog.String("providerType", fmt.Sprintf("%T", etcdProvider)))

	store := &BufferedEtcdStore{
		etcd: etcdProvider,
	}
	slog.InfoContext(ctx, "创建runloop", slog.String("event", "NewBufferedEtcdStore"))
	store.runloop = krunloop.NewRunLoop(ctx, store, "etcdstore")
	go store.runloop.Run(ctx)
	slog.InfoContext(ctx, "BufferedEtcdStore创建完成", slog.String("event", "NewBufferedEtcdStore"))
	store.initMetrics(ctx)
	return store
}

func (store *BufferedEtcdStore) Put(ctx context.Context, key string, value string, name string) {
	slog.DebugContext(ctx, "将写入请求加入队列", slog.String("event", "BufferedEtcdStorePut"), slog.String("key", key), slog.Int("valueLength", len(value)), slog.String("name", name))
	eve := NewWriteEvent(key, value, name)
	store.runloop.PostEvent(eve)
}

// BufferedEtcdStore implements CriticalResource
func (store *BufferedEtcdStore) IsResource() {}

func (store *BufferedEtcdStore) Shutdown(ctx context.Context) {
	slog.InfoContext(ctx, "关闭BufferedEtcdStore", slog.String("event", "BufferedEtcdStoreShutdown"))
	store.runloop.StopAndWaitForExit()
}

func (store *BufferedEtcdStore) initMetrics(ctx context.Context) {
	// 初始化指标
	EtcdStoreWriteSizeMetrics.GetTimeSequence(ctx, "WorkerState").Touch()
	EtcdStoreWriteSizeMetrics.GetTimeSequence(ctx, "RoutingEntry").Touch()
	EtcdStoreWriteSizeMetrics.GetTimeSequence(ctx, "PilotNode").Touch()
}

// WriteEvent implements IEvent[*BufferedEtcdStore]
type WriteEvent struct {
	createTimeMs int64 // time when the event was created
	Key          string
	Value        string
	Name         string // for logging/metrics purposes only
}

func NewWriteEvent(key string, value string, name string) *WriteEvent {
	return &WriteEvent{
		Key:          key,
		Value:        value,
		Name:         name,
		createTimeMs: kcommon.GetWallTimeMs(),
	}
}

func (eve *WriteEvent) GetCreateTimeMs() int64 {
	return eve.createTimeMs
}
func (eve *WriteEvent) GetName() string {
	return eve.Name
}

func (eve *WriteEvent) Process(ctx context.Context, resource *BufferedEtcdStore) {
	size := len(eve.Value) + len(eve.Key)
	EtcdStoreWriteSizeMetrics.GetTimeSequence(ctx, eve.Name).Add(int64(size))
	slog.DebugContext(ctx, "处理写入请求", slog.String("event", "WriteEventProcess"), slog.String("key", eve.Key), slog.Int("len", len(eve.Value)), slog.String("name", eve.Name))
	if eve.Value == "" {
		slog.InfoContext(ctx, "从etcd中删除键", slog.String("event", "WriteEventDelete"), slog.String("key", eve.Key))
		resource.etcd.Delete(ctx, eve.Key, false)
		return
	}
	slog.DebugContext(ctx, "向etcd写入数据", slog.String("event", "WriteEventProcess"), slog.String("key", eve.Key), slog.Int("len", len(eve.Value)))
	resource.etcd.Set(ctx, eve.Key, eve.Value)
}

// ResetEtcdStore 重置全局EtcdStore变量，用于测试
func ResetEtcdStore() {
	storeMutex.Lock()
	defer storeMutex.Unlock()
	if currentEtcdStore != nil {
		currentEtcdStore.Shutdown(context.Background())
		currentEtcdStore = nil
	}
	storeCreationCount = 0
	storeAccessCount = 0
	storeLastAccessCaller = ""
}
