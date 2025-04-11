package etcd

import (
	"context"
	"strings"
	"time"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	clientv3 "go.etcd.io/etcd/client/v3"
)

/********************** DefEtcdProvider **********************/

// DefEtcdProvider implements EtcdProvider
type DefEtcdProvider struct {
	etcdEndpoints []string
	client        *clientv3.Client
}

func NewDefEtcdProvider() *DefEtcdProvider {
	// 从环境变量获取配置
	endpoints := getEndpointsFromEnv()
	dialTimeoutMs := getDialTimeoutMsFromEnv()

	// 创建 etcd 客户端
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: time.Duration(dialTimeoutMs) * time.Millisecond,
	})
	if err != nil {
		ke := kerror.Create("EtcdConnectError", "failed to connect to etcd").
			WithErrorCode(kerror.EC_INTERNAL_ERROR).
			With("endpoints", strings.Join(endpoints, ",")).
			With("error", err.Error())
		panic(ke)
	}
	return &DefEtcdProvider{
		etcdEndpoints: endpoints,
		client:        cli,
	}
}

func (p *DefEtcdProvider) CreateEtcdSession() EtcdSession {
	return NewDefEtcdSession(p)
}
func (p *DefEtcdProvider) WatchByPrefix(ctx context.Context, pathPrefix string, revision EtcdRevision) chan EtcdKvItem {
	// 监听 etcd 的前缀
	ch := make(chan EtcdKvItem)
	go func() {
		defer close(ch)
		rch := p.client.Watch(ctx, pathPrefix, clientv3.WithRev(int64(revision)))
		for wr := range rch {
			for _, ev := range wr.Events {
				if ev.Type == clientv3.EventTypeDelete {
					ch <- EtcdKvItem{
						Key:         string(ev.Kv.Key),
						Value:       "",
						ModRevision: EtcdRevision(ev.Kv.ModRevision),
					}
				} else {
					ch <- EtcdKvItem{
						Key:         string(ev.Kv.Key),
						Value:       string(ev.Kv.Value),
						ModRevision: EtcdRevision(ev.Kv.ModRevision),
					}
				}
			}
		}
	}()
	return ch
}

// DefEtcdSession implements EtcdSession
func NewDefEtcdSession(p *DefEtcdProvider) *DefEtcdSession {
	lease, err := p.client.Grant(context.Background(), 60)
	if err != nil {
		ke := kerror.Wrap(err, "EtcdGrantError", "failed to grant lease", false).With("endpoints", strings.Join(p.etcdEndpoints, ","))
		panic(ke)
	}

	return &DefEtcdSession{
		provider: p,
		lease:    lease.ID,
	}
}

type DefEtcdSession struct {
	provider *DefEtcdProvider
	lease    clientv3.LeaseID
}

// DeleteNode implements EtcdSession.
func (d *DefEtcdSession) DeleteNode(key string) {
	panic("unimplemented")
}

// GetCurrentState implements EtcdSession.
func (d *DefEtcdSession) GetCurrentState() EtcdSessionState {
	panic("unimplemented")
}

// PutNode implements EtcdSession.
func (d *DefEtcdSession) PutNode(key string, value string) {
	panic("unimplemented")
}

// SetStateListener implements EtcdSession.
func (d *DefEtcdSession) SetStateListener(listener EtcdStateListener) {
	panic("unimplemented")
}
