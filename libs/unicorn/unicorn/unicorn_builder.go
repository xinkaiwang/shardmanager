package unicorn

import (
	"context"
	"os"
	"strings"

	"github.com/xinkaiwang/shardmanager/libs/unicorn/data"
	"github.com/xinkaiwang/shardmanager/libs/unicorn/unicornjson"
)

type UnicornBuilder struct {
	etcdEndpoints []string
	treeBuilder   RoutingTreeBuilder
}

func NewUnicornBuilder() *UnicornBuilder {
	return &UnicornBuilder{}
}
func (b *UnicornBuilder) Build(ctx context.Context) *Unicorn {
	if len(b.etcdEndpoints) == 0 {
		// read from env
		b.etcdEndpoints = getEndpointsFromEnv()
	}

	return NewUnicorn(ctx, b.etcdEndpoints, b.treeBuilder)
}

func (b *UnicornBuilder) WithEtcdEndpoint(etcdEndpoints []string) *UnicornBuilder {
	b.etcdEndpoints = etcdEndpoints
	return b
}

func (b *UnicornBuilder) WithRoutingTreeBuilder(treeBuilder RoutingTreeBuilder) *UnicornBuilder {
	b.treeBuilder = treeBuilder
	return b
}

type RoutingTreeBuilder func(map[data.WorkerId]*unicornjson.WorkerEntryJson) RoutingTree

// getEndpointsFromEnv 从环境变量获取 etcd 端点配置
// 返回：
// - 如果设置了 UNICORN_ETCD_ENDPOINTS 环境变量，返回解析后的端点列表
// - 否则返回默认值 ["localhost:2379"]
func getEndpointsFromEnv() []string {
	if endpoints := os.Getenv("UNICORN_ETCD_ENDPOINTS"); endpoints != "" {
		return strings.Split(endpoints, ",")
	}
	return []string{"localhost:2379"}
}
