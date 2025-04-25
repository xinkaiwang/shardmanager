package cougar

import (
	"context"
)

/********************** CougarBuilder **********************/

type CougarBuilder struct {
	etcdEndpoint string
	notifyChange NotifyChangeFunc
	workerInfo   *WorkerInfo
}

func NewCougarBuilder() *CougarBuilder {
	return &CougarBuilder{}
}

func (b *CougarBuilder) Build(ctx context.Context) Cougar {
	return NewCougarImpl(ctx, b.etcdEndpoint, b.notifyChange, b.workerInfo)
}

func (b *CougarBuilder) WithEtcdEndpoint(etcdEndpoing string) *CougarBuilder {
	b.etcdEndpoint = etcdEndpoing
	return b
}

func (b *CougarBuilder) WithNotifyShardChange(notifyChange NotifyChangeFunc) *CougarBuilder {
	b.notifyChange = notifyChange
	return b
}

func (b *CougarBuilder) WithWorkerInfo(workerInfo *WorkerInfo) *CougarBuilder {
	b.workerInfo = workerInfo
	return b
}
