package provider

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// etcdDefaultProvider 是默认的 etcd 客户端实现
// 支持通过环境变量配置：
// - ETCD_ENDPOINTS: etcd 服务器地址，多个地址用逗号分隔，默认为 "localhost:2379"
// - ETCD_DIAL_TIMEOUT: 连接超时时间（秒），默认为 5
type etcdDefaultProvider struct {
	client *clientv3.Client
}

// NewDefaultEtcdProvider 创建一个新的 etcd 客户端实例
// 错误处理：
// - 如果无法连接到 etcd 服务器，将返回 EtcdConnectError
// - 如果环境变量配置无效，将使用默认值
func NewDefaultEtcdProvider(_ context.Context) EtcdProvider {
	// 从环境变量获取配置
	endpoints := getEndpointsFromEnv()
	dialTimeout := getDialTimeoutFromEnv()

	// 创建 etcd 客户端
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: time.Duration(dialTimeout) * time.Second,
	})
	if err != nil {
		panic(kerror.Create("EtcdConnectError", "failed to connect to etcd").
			WithErrorCode(kerror.EC_INTERNAL_ERROR).
			With("endpoints", strings.Join(endpoints, ",")).
			With("error", err.Error()))
	}
	return &etcdDefaultProvider{
		client: cli,
	}
}

// Get 获取指定键的值
// 参数：
// - ctx: 上下文，用于取消操作
// - key: 要获取的键
// 返回：
// - 如果键存在，返回包含键值和版本信息的 EtcdKvItem
// - 如果键不存在，返回空值的 EtcdKvItem
// 错误处理：
// - 如果发生网络错误或其他 etcd 错误，将返回 EtcdGetError
func (pvd *etcdDefaultProvider) Get(ctx context.Context, key string) EtcdKvItem {
	resp, err := pvd.client.Get(ctx, key)
	if err != nil {
		panic(kerror.Create("EtcdGetError", "failed to get key from etcd").
			WithErrorCode(kerror.EC_INTERNAL_ERROR).
			With("key", key).
			With("error", err.Error()))
	}

	// 键不存在时返回空值
	if len(resp.Kvs) == 0 {
		return EtcdKvItem{
			Key:         key,
			Value:       "",
			ModRevision: 0,
		}
	}

	kv := resp.Kvs[0]
	return EtcdKvItem{
		Key:         string(kv.Key),
		Value:       string(kv.Value),
		ModRevision: kv.ModRevision,
	}
}

// List 列出指定前缀的键值对
// 参数：
// - ctx: 上下文，用于取消操作
// - startKey: 起始键（前缀）
// - maxCount: 最大返回数量，如果为 0 则返回所有匹配的键
// 返回：
// - 匹配前缀的键值对列表
// 错误处理：
// - 如果发生网络错误或其他 etcd 错误，将返回 EtcdListError
func (pvd *etcdDefaultProvider) List(ctx context.Context, startKey string, maxCount int) []EtcdKvItem {
	opts := []clientv3.OpOption{
		clientv3.WithPrefix(),
		clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend),
	}
	if maxCount > 0 {
		opts = append(opts, clientv3.WithLimit(int64(maxCount)))
	}

	klogging.Info(ctx).
		With("startKey", startKey).
		With("maxCount", maxCount).
		Log("ListKeysRequest", "listing keys from etcd")

	resp, err := pvd.client.Get(ctx, startKey, opts...)
	if err != nil {
		panic(kerror.Create("EtcdListError", "failed to list keys from etcd").
			WithErrorCode(kerror.EC_INTERNAL_ERROR).
			With("startKey", startKey).
			With("error", err.Error()))
	}

	klogging.Info(ctx).
		With("startKey", startKey).
		With("count", len(resp.Kvs)).
		Log("ListKeysResponse", "got keys from etcd")

	items := make([]EtcdKvItem, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		items = append(items, EtcdKvItem{
			Key:         string(kv.Key),
			Value:       string(kv.Value),
			ModRevision: kv.ModRevision,
		})
		klogging.Debug(ctx).
			With("key", string(kv.Key)).
			With("value", string(kv.Value)).
			Log("ListKeysItem", "found key")
	}
	return items
}

// Set 设置指定键的值
// 参数：
// - ctx: 上下文，用于取消操作
// - key: 要设置的键
// - value: 要设置的值
// 错误处理：
// - 如果发生网络错误或其他 etcd 错误，将返回 EtcdPutError
func (pvd *etcdDefaultProvider) Set(ctx context.Context, key, value string) {
	_, err := pvd.client.Put(ctx, key, value)
	if err != nil {
		panic(kerror.Create("EtcdPutError", "failed to set key in etcd").
			WithErrorCode(kerror.EC_INTERNAL_ERROR).
			With("key", key).
			With("error", err.Error()))
	}
}

// Delete 删除指定的键
// 参数：
// - ctx: 上下文，用于取消操作
// - key: 要删除的键
// 错误处理：
// - 如果发生网络错误或其他 etcd 错误，将返回 EtcdDeleteError
// - 如果键不存在，将返回 KeyNotFound
func (pvd *etcdDefaultProvider) Delete(ctx context.Context, key string) {
	resp, err := pvd.client.Delete(ctx, key)
	if err != nil {
		panic(kerror.Create("EtcdDeleteError", "failed to delete key from etcd").
			WithErrorCode(kerror.EC_INTERNAL_ERROR).
			With("key", key).
			With("error", err.Error()))
	}

	// 删除时键必须存在
	if resp.Deleted == 0 {
		panic(kerror.Create("KeyNotFound", "key not found in etcd").
			WithErrorCode(kerror.EC_NOT_FOUND).
			With("key", key))
	}
}

// getEndpointsFromEnv 从环境变量获取 etcd 端点配置
// 返回：
// - 如果设置了 ETCD_ENDPOINTS 环境变量，返回解析后的端点列表
// - 否则返回默认值 ["localhost:2379"]
func getEndpointsFromEnv() []string {
	if endpoints := os.Getenv("ETCD_ENDPOINTS"); endpoints != "" {
		return strings.Split(endpoints, ",")
	}
	return []string{"localhost:2379"}
}

// getDialTimeoutFromEnv 从环境变量获取连接超时配置
// 返回：
// - 如果设置了 ETCD_DIAL_TIMEOUT 环境变量且为有效整数，返回该值
// - 否则返回默认值 5（秒）
func getDialTimeoutFromEnv() int {
	if timeout := os.Getenv("ETCD_DIAL_TIMEOUT"); timeout != "" {
		if value, err := time.ParseDuration(timeout); err == nil {
			return int(value.Seconds())
		}
	}
	return 5
}
