package provider

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

// EtcdProvider 定义了 etcd 操作的接口
type EtcdProvider interface {
	// Get 获取指定键的值
	Get(ctx context.Context, key string) EtcdKvItem

	// List 列出指定前缀的键值对
	// prefix: 前缀
	// limit: 每页返回的最大记录数，0表示不限制
	// nextToken: 上一页的最后一个key，用于分页，空表示从头开始
	// 返回值: 当前页的键值对列表和下一页的起始key(如果还有更多)
	List(ctx context.Context, prefix string, limit int, nextToken string) ([]EtcdKvItem, string)

	// Set 设置指定键的值
	Set(ctx context.Context, key, value string)

	// Delete 删除指定的键
	Delete(ctx context.Context, key string)
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
