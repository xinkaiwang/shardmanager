package etcdprov

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"

	clientv3 "go.etcd.io/etcd/client/v3"
)

var (
	etcdQueryTimeoutMs = kcommon.GetEnvInt("ETCD_TIMEOUT_MS", 3*1000)
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

// LoadAllByPrefix
func (pvd *etcdDefaultProvider) LoadAllByPrefix(ctx context.Context, pathPrefix string) ([]EtcdKvItem, EtcdRevision) {
	var items []EtcdKvItem
	const pageSize = 1000
	ctxTimeout, cancel := context.WithTimeout(ctx, time.Duration(etcdQueryTimeoutMs)*time.Millisecond)
	defer cancel()

	// 首先获取当前的版本号，后续所有查询都使用这个版本号
	resp, err := pvd.client.Get(ctxTimeout, pathPrefix, clientv3.WithPrefix(), clientv3.WithLimit(1))
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			ke := kerror.Create("EtcdTimeout", "etcd get operation timed out").
				WithErrorCode(kerror.EC_TIMEOUT).
				With("pathPrefix", pathPrefix).
				With("timeoutMs", etcdQueryTimeoutMs).
				With("error", err.Error())
			panic(ke)
		}
		ke := kerror.Create("EtcdLoadError", "failed to get initial revision").
			WithErrorCode(kerror.EC_INTERNAL_ERROR).
			With("pathPrefix", pathPrefix).
			With("error", err.Error())
		panic(ke)
	}
	revision := resp.Header.Revision

	// 记录开始加载的版本号
	klogging.Debug(ctx).
		With("pathPrefix", pathPrefix).
		With("revision", revision).
		Log("LoadAllByPrefix", "starting load at revision")

	var key string = pathPrefix
	for {
		// 构建查询选项，始终使用相同的 revision
		opts := []clientv3.OpOption{
			clientv3.WithPrefix(),
			clientv3.WithLimit(pageSize),
			clientv3.WithRev(revision), // 关键：使用相同的 revision
		}

		// 如果不是第一页，设置范围
		if key != pathPrefix {
			opts = append(opts, clientv3.WithFromKey())
			rangeEnd := clientv3.GetPrefixRangeEnd(pathPrefix)
			opts = append(opts, clientv3.WithRange(rangeEnd))
		}

		// 执行查询
		resp, err := pvd.client.Get(ctxTimeout, key, opts...)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				ke := kerror.Create("EtcdTimeout", "etcd get operation timed out").
					WithErrorCode(kerror.EC_TIMEOUT).
					With("pathPrefix", pathPrefix).
					With("timeoutMs", etcdQueryTimeoutMs).
					With("error", err.Error())
				panic(ke)
			}
			ke := kerror.Create("EtcdLoadError", "failed to get initial revision").
				WithErrorCode(kerror.EC_INTERNAL_ERROR).
				With("pathPrefix", pathPrefix).
				With("error", err.Error())
			panic(ke)
		}

		// 记录本次加载的数量
		klogging.Debug(ctx).
			With("pathPrefix", pathPrefix).
			With("pageCount", len(resp.Kvs)).
			With("totalCount", len(items)+len(resp.Kvs)).
			With("revision", revision).
			Log("LoadAllByPrefix", "loaded page of keys from etcd")

		// 如果没有更多数据，退出循环
		if len(resp.Kvs) == 0 {
			break
		}

		// 处理本页数据
		for _, kv := range resp.Kvs {
			// 跳过起始键（除了第一页）
			if string(kv.Key) == key && key != pathPrefix {
				continue
			}

			items = append(items, EtcdKvItem{
				Key:         string(kv.Key),
				Value:       string(kv.Value),
				ModRevision: EtcdRevision(kv.ModRevision),
			})
		}

		// 如果本页数据量小于 pageSize，说明已经是最后一页
		if len(resp.Kvs) < pageSize {
			break
		}

		// 更新 key，准备加载下一页
		key = string(resp.Kvs[len(resp.Kvs)-1].Key)
	}

	for _, kv := range items {
		klogging.Info(ctx).
			With("key", kv.Key).
			With("value", kv.Value).
			Log("LoadAllByPrefix", "found key")
	}
	return items, EtcdRevision(revision)
}

// WatchByPrefix
func (pvd *etcdDefaultProvider) WatchByPrefix(ctx context.Context, pathPrefix string, revision EtcdRevision) chan EtcdKvItem {
	eventChan := make(chan EtcdKvItem, 100)

	go func() {
		defer close(eventChan)

		// 记录当前监听的版本号
		currentRev := revision

		for {
			// 检查上下文是否已取消
			if ctx.Err() != nil {
				klogging.Info(ctx).
					With("pathPrefix", pathPrefix).
					With("revision", currentRev).
					Log("WatchByPrefix", "context cancelled, stopping watch")
				return
			}

			// 设置 watch 选项
			opts := []clientv3.OpOption{
				clientv3.WithPrefix(),
				clientv3.WithRev(int64(currentRev)), // 从指定版本开始
			}

			// 开始监听
			klogging.Info(ctx).
				With("pathPrefix", pathPrefix).
				With("revision", currentRev).
				Log("WatchByPrefix", "starting watch")

			watchChan := pvd.client.Watch(ctx, pathPrefix, opts...)

			// 处理 watch 事件
			for wresp := range watchChan {
				// 检查是否有错误
				if wresp.Err() != nil {
					klogging.Error(ctx).
						With("pathPrefix", pathPrefix).
						With("revision", currentRev).
						With("error", wresp.Err()).
						Log("WatchByPrefix", "watch error occurred")
					break // 跳出内层循环，重新建立 watch
				}

				// 检查是否是压缩版本错误
				if wresp.CompactRevision > 0 {
					klogging.Warning(ctx).
						With("pathPrefix", pathPrefix).
						With("requestedRevision", currentRev).
						With("compactRevision", wresp.CompactRevision).
						Log("WatchByPrefix", "requested revision has been compacted")
					// 从压缩版本开始重新监听
					currentRev = EtcdRevision(wresp.CompactRevision)
					break // 跳出内层循环，使用新的 revision 重新建立 watch
				}

				// 更新当前版本号（为了在重连时使用）
				if len(wresp.Events) > 0 {
					currentRev = EtcdRevision(wresp.Events[len(wresp.Events)-1].Kv.ModRevision + 1)
				}

				// 处理事件
				for _, event := range wresp.Events {
					item := EtcdKvItem{
						Key:         string(event.Kv.Key),
						ModRevision: EtcdRevision(event.Kv.ModRevision),
					}

					switch event.Type {
					case clientv3.EventTypePut:
						item.Value = string(event.Kv.Value)
						klogging.Debug(ctx).
							With("key", item.Key).
							With("revision", item.ModRevision).
							Log("WatchByPrefix", "key updated")
					case clientv3.EventTypeDelete:
						item.Value = ""
						klogging.Debug(ctx).
							With("key", item.Key).
							With("revision", item.ModRevision).
							Log("WatchByPrefix", "key deleted")
					}

					// 发送事件，如果上下文已取消则退出
					select {
					case eventChan <- item:
					case <-ctx.Done():
						return
					}
				}
			}

			// 如果上下文已取消，退出
			if ctx.Err() != nil {
				return
			}

			// 记录重连尝试
			klogging.Warning(ctx).
				With("pathPrefix", pathPrefix).
				With("revision", currentRev).
				Log("WatchByPrefix", "watch channel closed, retrying")

			// 添加短暂延迟避免立即重试
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
				// 继续外层循环，重新建立 watch
			}
		}
	}()

	return eventChan
}
