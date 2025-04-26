package cougar

import (
	"context"
)

/********************** CougarBuilder **********************/

type CougarBuilder struct {
	etcdEndpoint string
	workerInfo   *WorkerInfo
	cougarApp    CougarApp
}

func NewCougarBuilder() *CougarBuilder {
	return &CougarBuilder{}
}

func (b *CougarBuilder) Build(ctx context.Context) Cougar {
	return NewCougarImpl(ctx, b.etcdEndpoint, b.workerInfo, b.cougarApp)
}

func (b *CougarBuilder) WithEtcdEndpoint(etcdEndpoing string) *CougarBuilder {
	b.etcdEndpoint = etcdEndpoing
	return b
}

func (b *CougarBuilder) WithWorkerInfo(workerInfo *WorkerInfo) *CougarBuilder {
	b.workerInfo = workerInfo
	return b
}

func (b *CougarBuilder) WithCougarApp(cougarApp CougarApp) *CougarBuilder {
	b.cougarApp = cougarApp
	return b
}
