package etcdprov

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
)

var (
	etcdTimeoutMs      = kcommon.GetEnvInt("UNICORN_ETCD_TIMEOUT_MS", 3*1000)
	etcdLeaseTimeoutMs = kcommon.GetEnvInt("UNICORN_ETCD_LEASE_TIMEOUT_MS", 15*1000)

	currentEtcdProvider EtcdProvider
)

func GetCurrentEtcdProvider(ctx context.Context) EtcdProvider {
	if currentEtcdProvider == nil {
		currentEtcdProvider = NewDefaultEtcdProvider(ctx)
	}
	return currentEtcdProvider
}

// EtcdKvItem 表示一个 etcd 键值对
type EtcdKvItem struct {
	Key         string
	Value       string
	ModRevision EtcdRevision
}

type EtcdRevision int64

type EtcdProvider interface {
	// LoadAllByPrefix
	LoadAllByPrefix(ctx context.Context, pathPrefix string) ([]EtcdKvItem, EtcdRevision)

	// WatchPrefix
	WatchByPrefix(ctx context.Context, pathPrefix string, revision EtcdRevision) chan EtcdKvItem
}
