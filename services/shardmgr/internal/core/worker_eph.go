package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/services/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/etcdprov"
)

type WorkerEphWatcher struct {
	ss *ServiceState
	ch chan *cougarjson.WorkerEphJson
}

func NewWorkerEphWatcher(ctx context.Context, ss *ServiceState, currentWorkerEphRevision etcdprov.EtcdRevision) *WorkerEphWatcher {
	watcher := &WorkerEphWatcher{
		ss: ss,
		ch: make(chan *cougarjson.WorkerEphJson),
	}
	go watcher.Run(ctx)
	return watcher
}

func (w *WorkerEphWatcher) Run(ctx context.Context) {
	// TODO
}

// syncWorkerEph: must be called in runloop
func (ss *ServiceState) syncWorkerEph(ctx context.Context, workerEph *cougarjson.WorkerEphJson) {
	// TODO
}

func (ss *ServiceState) LoadCurrentWorkerEph(ctx context.Context) ([]*cougarjson.WorkerEphJson, etcdprov.EtcdRevision) {
	list, version := etcdprov.GetCurrentEtcdProvider(ctx).LoadAllByPrefix(ctx, ss.PathManager.GetWorkerEphPathPrefix())
	workerEphs := []*cougarjson.WorkerEphJson{}
	for _, item := range list {
		workerEphs = append(workerEphs, cougarjson.WorkerEphJsonFromJson(item.Value))
	}
	return workerEphs, version
}
