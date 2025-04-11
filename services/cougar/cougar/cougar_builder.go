package cougar

/********************** CougarBuilder **********************/

type CougarBuilder struct {
	etcdEndpoint string
	notifyChange NotifyChangeFunc
	workerInfo   *WorkerInfo
}

func NewCougarBuilder() *CougarBuilder {
	return &CougarBuilder{}
}

func (b *CougarBuilder) Build() Cougar {
	return NewCougarImpl(b.etcdEndpoint, b.notifyChange, b.workerInfo)
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

/********************** CougarImpl **********************/
type CougarImpl struct {
	etcdEndpoint string
	notifyChange NotifyChangeFunc
	workerInfo   *WorkerInfo
}

func NewCougarImpl(etcdEndpoint string, notifyChange NotifyChangeFunc, workerInfo *WorkerInfo) *CougarImpl {
	return &CougarImpl{
		etcdEndpoint: etcdEndpoint,
		notifyChange: notifyChange,
		workerInfo:   workerInfo,
	}
}

func (c *CougarImpl) VisitState(visitor CougarStateVisitor) {
	// TODO
}

func (c *CougarImpl) RequestShutdown() {
	// TODO
}
