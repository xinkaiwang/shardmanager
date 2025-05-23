package provider

import (
	"context"
	"encoding/base64"
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
// - prefix: 前缀
// - limit: 每页返回的最大记录数，0表示不限制
// - nextToken: 上一页的最后一个key的base64编码，用于分页，空表示从头开始
// 返回：
// - 当前页的键值对列表和下一页的起始key的base64编码(如果还有更多)
// 错误处理：
// - 如果发生网络错误或其他 etcd 错误，将返回 EtcdListError
func (pvd *etcdDefaultProvider) List(ctx context.Context, prefix string, limit int, nextToken string) ([]EtcdKvItem, string) {
	// 构建基本查询选项
	opts := []clientv3.OpOption{
		clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend),
	}

	// 处理 nextToken
	var startKey string
	var useNextToken bool

	if nextToken != "" {
		// base64解码nextToken
		decodedBytes, err := base64.StdEncoding.DecodeString(nextToken)
		if err != nil {
			panic(kerror.Create("InvalidNextToken", "failed to decode next token").
				WithErrorCode(kerror.EC_INVALID_PARAMETER).
				With("hasNextToken", nextToken != "").
				With("error", err.Error()))
		}

		// 正确处理键，确保不再返回token键本身
		decodedKey := string(decodedBytes)
		if decodedKey != "" {
			startKey = decodedKey
			useNextToken = true

			// 对于分页查询，我们需要查询的是比startKey大的键
			// 使用clientv3.WithFromKey()让查询从这个键的后继开始
			opts = append(opts, clientv3.WithFromKey())

			// 当有前缀时，限制查询范围到前缀范围内
			if prefix != "" {
				rangeEnd := clientv3.GetPrefixRangeEnd(prefix)
				opts = append(opts, clientv3.WithRange(rangeEnd))
			}
		}
	} else if prefix != "" {
		// 首次查询，使用前缀
		startKey = prefix
		opts = append(opts, clientv3.WithPrefix())
	} else {
		// 没有前缀，从最小的可能键开始查询所有键
		startKey = "\x00" // 使用最小字节作为起始键
		opts = append(opts, clientv3.WithFromKey())
	}

	// 设置限制(多请求一个用于判断是否有下一页)
	if limit > 0 {
		opts = append(opts, clientv3.WithLimit(int64(limit+1)))
	}

	klogging.Info(ctx).
		With("prefix", prefix).
		With("limit", limit).
		With("hasNextToken", nextToken != "").
		With("useNextToken", useNextToken).
		Log("ListKeysRequest", "listing keys from etcd")

	// 查询etcd
	resp, err := pvd.client.Get(ctx, startKey, opts...)
	if err != nil {
		panic(kerror.Create("EtcdListError", "failed to list keys from etcd").
			WithErrorCode(kerror.EC_INTERNAL_ERROR).
			With("prefix", prefix).
			With("hasStartKey", startKey != "").
			With("error", err.Error()))
	}

	// 结果为空时直接返回
	if len(resp.Kvs) == 0 {
		return []EtcdKvItem{}, ""
	}

	// 当使用nextToken时，我们需要跳过第一个结果（这就是token key本身）
	startIndex := 0
	if useNextToken && len(resp.Kvs) > 0 && string(resp.Kvs[0].Key) == startKey {
		startIndex = 1
	}

	var items []EtcdKvItem
	var next string

	// 处理分页 - 限制结果数量并计算下一页token
	actualLimit := limit
	if actualLimit <= 0 {
		actualLimit = len(resp.Kvs) - startIndex // 如果没有限制，使用实际结果数
	}

	// 如果结果数量超过限制，取出多余的一个用于nextToken
	totalAvailable := len(resp.Kvs) - startIndex
	hasMore := totalAvailable > actualLimit
	resultCount := actualLimit
	if hasMore {
		nextKeyIndex := startIndex + resultCount
		if nextKeyIndex < len(resp.Kvs) {
			nextKey := string(resp.Kvs[nextKeyIndex].Key)
			// base64编码nextToken
			next = base64.StdEncoding.EncodeToString([]byte(nextKey))
		}
	}

	// 构建结果，不超过限制
	items = make([]EtcdKvItem, 0, resultCount)
	endIndex := startIndex + resultCount
	if endIndex > len(resp.Kvs) {
		endIndex = len(resp.Kvs)
	}

	for i := startIndex; i < endIndex; i++ {
		kv := resp.Kvs[i]
		keyStr := string(kv.Key)

		// 确保结果与前缀匹配（对于使用WithFromKey的查询很重要）
		if prefix != "" && !strings.HasPrefix(keyStr, prefix) {
			continue
		}

		items = append(items, EtcdKvItem{
			Key:         keyStr,
			Value:       string(kv.Value),
			ModRevision: kv.ModRevision,
		})
	}

	klogging.Info(ctx).
		With("prefix", prefix).
		With("count", len(items)).
		With("hasMore", next != "").
		Log("ListKeysResponse", "got keys from etcd")

	return items, next
}

// Set 设置指定键的值
// 参数：
// - ctx: 上下文，用于取消操作
// - key: 要设置的键
// - value: 要设置的值
// 错误处理：
// - 如果发生网络错误或其他 etcd 错误，将返回 EtcdPutError
func (pvd *etcdDefaultProvider) Set(ctx context.Context, key, value string) {
	klogging.Info(ctx).With("key", key).With("valueLength", len(value)).Log("etcdDefaultProvider", "setting key in etcd")
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
