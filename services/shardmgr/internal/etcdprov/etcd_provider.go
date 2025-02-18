package etcdprov

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
)

// ErrKeyNotFound 表示键不存在
var ErrKeyNotFound = kerror.Create("KeyNotFound", "key not found").
	WithErrorCode(kerror.EC_NOT_FOUND)

// EtcdKvItem 表示一个 etcd 键值对
type EtcdKvItem struct {
	Key         string
	Value       string
	ModRevision int64
}

type EtcdRevision int64

// EtcdProvider 定义了 etcd 操作的接口
type EtcdProvider interface {
	// Get 获取指定键的值
	Get(ctx context.Context, key string) EtcdKvItem

	// List 列出指定前缀的键值对
	// maxCount 为 0 时返回所有匹配的键
	List(ctx context.Context, startKey string, maxCount int) []EtcdKvItem

	// Set 设置指定键的值
	Set(ctx context.Context, key, value string)

	// Delete 删除指定的键
	Delete(ctx context.Context, key string)

	// LoadAllByPrefix
	LoadAllByPrefix(ctx context.Context, pathPrefix string) ([]EtcdKvItem, EtcdRevision)

	// WatchPrefix
	WatchByPrefix(ctx context.Context, pathPrefix string, revision EtcdRevision) chan *EtcdKvItem
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
