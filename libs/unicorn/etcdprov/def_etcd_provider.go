package etcdprov

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"log/slog"

	clientv3 "go.etcd.io/etcd/client/v3"
)

var (
	etcdQueryTimeoutMs = kcommon.GetEnvInt("UNICORN_ETCD_TIMEOUT_MS", 5*1000)
)

// etcdDefaultProvider 是默认的 etcd 客户端实现
// 支持通过环境变量配置：
// - UNICORN_ETCD_ENDPOINTS: etcd 服务器地址，多个地址用逗号分隔，默认为 "localhost:2379"
// - UNICORN_ETCD_TIMEOUT_MS: 连接超时时间（毫秒），默认为 5000
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
// - 如果设置了 UNICORN_ETCD_ENDPOINTS 环境变量，返回解析后的端点列表
// - 否则返回默认值 ["localhost:2379"]
func getEndpointsFromEnv() []string {
	endpoints := kcommon.GetEnvString("UNICORN_ETCD_ENDPOINTS", "localhost:2379")
	return strings.Split(endpoints, ",")
}

// getDialTimeoutFromEnv 从环境变量获取连接超时配置
// 返回：
// - 如果设置了 UNICORN_ETCD_TIMEOUT_MS 环境变量且为有效整数，返回该值（毫秒）
// - 否则返回默认值 5（秒）
func getDialTimeoutFromEnv() int {
	if timeout := os.Getenv("UNICORN_ETCD_TIMEOUT_MS"); timeout != "" {
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
	slog.DebugContext(ctx, "starting load at revision",
		slog.String("event", "LoadAllByPrefix"),
		slog.String("pathPrefix", pathPrefix),
		slog.Int64("revision", int64(revision)))

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
		slog.DebugContext(ctx, "loaded page of keys from etcd",
		slog.String("event", "LoadAllByPrefix"),
		slog.String("pathPrefix", pathPrefix),
		slog.Int("pageCount", len(resp.Kvs)),
		slog.Int("totalCount", len(items)+len(resp.Kvs)),
		slog.Int64("revision", int64(revision)))

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
		slog.InfoContext(ctx, "found key",
		slog.String("event", "LoadAllByPrefix"),
		slog.String("key", kv.Key),
		slog.String("value", kv.Value))
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
				slog.InfoContext(ctx, "context cancelled, stopping watch",
		slog.String("event", "WatchByPrefix"),
		slog.String("pathPrefix", pathPrefix),
		slog.Int64("revision", int64(currentRev)))
				return
			}

			// 设置 watch 选项
			opts := []clientv3.OpOption{
				clientv3.WithPrefix(),
				clientv3.WithRev(int64(currentRev)), // 从指定版本开始
			}

			// 开始监听
			slog.InfoContext(ctx, "starting watch",
		slog.String("event", "WatchByPrefix"),
		slog.String("pathPrefix", pathPrefix),
		slog.Int64("revision", int64(currentRev)))

			watchChan := pvd.client.Watch(ctx, pathPrefix, opts...)

			// 处理 watch 事件
			for wresp := range watchChan {
				// 检查是否有错误
				if wresp.Err() != nil {
					slog.ErrorContext(ctx, "watch error occurred",
						slog.String("event", "WatchByPrefix.Error"),
						slog.String("pathPrefix", pathPrefix),
						slog.Any("revision", currentRev),
						slog.Any("error", wresp.Err()))
					break // 跳出内层循环，重新建立 watch
				}

				// 检查是否是压缩版本错误
				if wresp.CompactRevision > 0 {
					slog.WarnContext(ctx, "requested revision has been compacted",
						slog.String("event", "WatchByPrefix.Compacted"),
						slog.String("pathPrefix", pathPrefix),
						slog.Any("requestedRevision", currentRev),
						slog.Int64("compactRevision", wresp.CompactRevision))
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
						slog.DebugContext(ctx, "key updated",
							slog.String("event", "WatchByPrefix.Updated"),
							slog.String("key", item.Key),
							slog.Any("revision", item.ModRevision))
					case clientv3.EventTypeDelete:
						item.Value = ""
						slog.DebugContext(ctx, "key deleted",
							slog.String("event", "WatchByPrefix.Deleted"),
							slog.String("key", item.Key),
							slog.Any("revision", item.ModRevision))
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
			slog.WarnContext(ctx, "watch channel closed, retrying",
				slog.String("event", "WatchByPrefix.Retry"),
				slog.String("pathPrefix", pathPrefix),
				slog.Any("revision", currentRev))

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
