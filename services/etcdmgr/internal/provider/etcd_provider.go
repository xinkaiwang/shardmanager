package provider

import "context"

type EtcdProvider interface {
	Get(ctx context.Context, key string) EtcdKvItem
	Set(ctx context.Context, key, value string)
	List(ctx context.Context, startKey string, maxCount int) []EtcdKvItem
	Delete(ctx context.Context, key string)
}

type EtcdKvItem struct {
	Key         string
	Value       string
	ModRevision int64
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
