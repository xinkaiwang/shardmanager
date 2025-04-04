package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/etcdprov"
)

type WorkerEphWatcher struct {
	ss *ServiceState
	ch chan etcdprov.EtcdKvItem
}

func NewWorkerEphWatcher(ctx context.Context, ss *ServiceState, currentWorkerEphRevision etcdprov.EtcdRevision) *WorkerEphWatcher {
	watcher := &WorkerEphWatcher{
		ss: ss,
	}
	watcher.ch = etcdprov.GetCurrentEtcdProvider(ctx).WatchByPrefix(ctx, ss.PathManager.GetWorkerEphPathPrefix(), currentWorkerEphRevision)
	go watcher.Run(ctx)
	return watcher
}

func (w *WorkerEphWatcher) Run(ctx context.Context) {
	klogging.Info(ctx).Log("WorkerEphWatcherStarted", "exit")
	for {
		select {
		case <-ctx.Done():
			klogging.Info(ctx).Log("WorkerEphWatcherExit", "exit")
			return
		case kvItem := <-w.ch:
			if kvItem.Value == "" {
				// this is a delete event
				str := kvItem.Key[len(w.ss.PathManager.GetWorkerEphPathPrefix()):] // exclude prefix '/smg/eph/'
				klogging.Info(ctx).With("workerFullId", str).
					Log("WorkerEphWatcher", "观察到worker eph已删除")
				w.ss.PostEvent(NewWorkerEphEvent(data.WorkerFullIdParseFromString(str), nil))
				continue
			}
			// this is a add or update event
			klogging.Info(ctx).With("workerFullId", kvItem.Key).With("eph", kvItem.Value).
				Log("WorkerEphWatcher", "观察到worker eph已更新")
			str := kvItem.Key[len(w.ss.PathManager.GetWorkerEphPathPrefix()):] // exclude prefix '/smg/eph/'
			workerFullId := data.WorkerFullIdParseFromString(str)
			workerEph := cougarjson.WorkerEphJsonFromJson(kvItem.Value)
			w.ss.PostEvent(NewWorkerEphEvent(workerFullId, workerEph))
		}
	}
}

func (ss *ServiceState) LoadCurrentWorkerEph(ctx context.Context) ([]*cougarjson.WorkerEphJson, etcdprov.EtcdRevision) {
	list, version := etcdprov.GetCurrentEtcdProvider(ctx).LoadAllByPrefix(ctx, ss.PathManager.GetWorkerEphPathPrefix())
	workerEphs := []*cougarjson.WorkerEphJson{}
	for _, item := range list {
		workerEphs = append(workerEphs, cougarjson.WorkerEphJsonFromJson(item.Value))
	}
	return workerEphs, version
}

// WorkerEphEvent: implements IEvent[*ServiceState]
type WorkerEphEvent struct {
	WorkerFullId data.WorkerFullId
	WorkerEph    *cougarjson.WorkerEphJson
}

func NewWorkerEphEvent(workerFullId data.WorkerFullId, workerEph *cougarjson.WorkerEphJson) *WorkerEphEvent {
	return &WorkerEphEvent{
		WorkerFullId: workerFullId,
		WorkerEph:    workerEph,
	}
}

func (e *WorkerEphEvent) GetName() string {
	return "WorkerEphEvent"
}

func (e *WorkerEphEvent) Process(ctx context.Context, ss *ServiceState) {
	ss.writeWorkerEphToStaging(ctx, e.WorkerFullId, e.WorkerEph)
}

// writeWorkerEphToStaging: must be called in runloop
func (ss *ServiceState) writeWorkerEphToStaging(ctx context.Context, workerFullId data.WorkerFullId, workerEph *cougarjson.WorkerEphJson) {
	// klogging.Info(ctx).With("workerFullId", workerFullId.String()).
	// 	With("hasEph", workerEph != nil).With("eph", workerEph).
	// 	Log("writeWorkerEphToStaging", "开始处理worker eph")

	defer func() {
		ss.syncWorkerBatchManager.TrySchedule(ctx)
		// klogging.Info(ctx).With("workerFullId", workerFullId.String()).
		// 	With("dirtyCount", len(ss.EphDirty)).
		// 	Log("writeWorkerEphToStaging", "已安排BatchManager")
	}()

	if workerEph == nil {
		// delete
		delete(ss.EphWorkerStaging, workerFullId)
		ss.EphDirty[workerFullId] = common.Unit{}
		// klogging.Info(ctx).With("workerFullId", workerFullId.String()).
		// 	Log("writeWorkerEphToStaging", "worker eph已删除")
		return
	}

	if ss.EphWorkerStaging[workerFullId] == nil {
		// add
		ss.EphWorkerStaging[workerFullId] = workerEph
		ss.EphDirty[workerFullId] = common.Unit{}
		// klogging.Info(ctx).With("workerFullId", workerFullId.String()).
		// 	Log("writeWorkerEphToStaging", "worker eph已添加")
		return
	}

	// update
	ss.EphWorkerStaging[workerFullId] = workerEph
	ss.EphDirty[workerFullId] = common.Unit{}
	// klogging.Info(ctx).With("workerFullId", workerFullId.String()).
	// 	Log("writeWorkerEphToStaging", "worker eph已更新")
}
