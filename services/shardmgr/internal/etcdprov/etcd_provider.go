package etcdprov

import (
	"context"
	"fmt"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
)

// ErrKeyNotFound 表示键不存在
var ErrKeyNotFound = kerror.Create("KeyNotFound", "key not found").
	WithErrorCode(kerror.EC_NOT_FOUND)

// EtcdKvItem 表示一个 etcd 键值对
type EtcdKvItem struct {
	Key         string
	Value       string
	ModRevision EtcdRevision
}

type EtcdRevision int64

// EtcdProvider 定义了 etcd 操作的接口
type EtcdProvider interface {
	// Get 获取指定键的值, 如果键不存在，返回空值的 EtcdKvItem
	Get(ctx context.Context, key string) EtcdKvItem

	// List 列出指定前缀的键值对
	// maxCount 为 0 时返回所有匹配的键
	List(ctx context.Context, startKey string, maxCount int) []EtcdKvItem

	// Set 设置指定键的值
	Set(ctx context.Context, key, value string)

	// Delete 删除指定的键
	Delete(ctx context.Context, key string, strictMode bool)

	// LoadAllByPrefix
	LoadAllByPrefix(ctx context.Context, pathPrefix string) ([]EtcdKvItem, EtcdRevision)

	// WatchPrefix
	WatchByPrefix(ctx context.Context, pathPrefix string, revision EtcdRevision) chan EtcdKvItem
}

var (
	currentEtcdProvider EtcdProvider
)

func GetCurrentEtcdProvider(ctx context.Context) EtcdProvider {
	if currentEtcdProvider == nil {
		currentEtcdProvider = NewDefaultEtcdProvider(ctx)
	}
	return currentEtcdProvider
}

// RunWithEtcdProvider 在执行 fn 期间临时使用提供的 EtcdProvider，执行完成后恢复原来的 provider
// 无论 fn 是否 panic，都会确保恢复原来的 provider
func RunWithEtcdProvider(provider EtcdProvider, fn func()) {
	ctx := context.Background()
	oldProvider := currentEtcdProvider

	// 仅记录关键信息，并简化日志内容
	klogging.Info(ctx).Log("event", "RunWithEtcdProvider")

	currentEtcdProvider = provider
	defer func() {
		// 恢复原始的 provider
		currentEtcdProvider = oldProvider
	}()

	fn()
}

// DumpGlobalState 返回当前全局EtcdProvider的状态信息
func DumpGlobalState() string {
	ctx := context.Background()
	provider := GetCurrentEtcdProvider(ctx)
	return fmt.Sprintf("currentEtcdProvider: %T", provider)
}

// ResetEtcdProvider 重置全局EtcdProvider变量，用于测试
func ResetEtcdProvider() {
	currentEtcdProvider = nil
}
