package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
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
	go watcher.Run(klogging.EmbedTraceId(ctx, "wew_")) // wew = worker eph watcher
	return watcher
}

func (w *WorkerEphWatcher) Run(ctx context.Context) {
	klogging.Info(ctx).Log("WorkerEphWatcher", "Started")
	for {
		select {
		case <-ctx.Done():
			klogging.Info(ctx).Log("WorkerEphWatcher", "CtxDone.Exit")
			return
		case kvItem := <-w.ch:
			traceId := kcommon.NewTraceId(ctx, "wew_", 8) // wew = worker eph watcher
			ctx2 := klogging.EmbedTraceId(ctx, traceId)
			if kvItem.Value == "" {
				// this is a delete event
				// str := kvItem.Key[len(w.ss.PathManager.GetWorkerEphPathPrefix()):] // exclude prefix '/smg/eph/'
				workerId := w.workerIdFromPath(kvItem.Key)
				klogging.Info(ctx2).With("workerId", workerId).With("path", kvItem.Key).
					Log("WorkerEphWatcher", "观察到worker eph已删除")
				w.ss.PostEvent(NewWorkerEphEvent(ctx2, workerId, nil))
				// klogging.Info(ctx2).With("workerId", workerId).
				// 	Log("WorkerEphWatcher", "worker eph event posted")
				continue
			}
			// this is a add or update event
			klogging.Info(ctx2).With("workerFullId", kvItem.Key).WithDebug("eph", kvItem.Value).
				Log("WorkerEphWatcher", "观察到worker eph已更新")
			// str := kvItem.Key[len(w.ss.PathManager.GetWorkerEphPathPrefix()):] // exclude prefix '/smg/eph/'
			workerId := w.workerIdFromPath(kvItem.Key)
			workerEph := cougarjson.WorkerEphJsonFromJson(kvItem.Value)
			w.ss.PostEvent(NewWorkerEphEvent(ctx2, workerId, workerEph))
			// klogging.Info(ctx2).With("workerId", workerId).With("eph", workerEph).
			// 	Log("WorkerEphWatcher", "worker eph event posted")
		}
	}
}

func (w *WorkerEphWatcher) workerIdFromPath(path string) data.WorkerId {
	// exclude prefix '/smg/eph/'
	return data.WorkerFullIdParseFromString(path[len(w.ss.PathManager.GetWorkerEphPathPrefix()):]).WorkerId
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
	Ctx       context.Context
	WorkerId  data.WorkerId
	WorkerEph *cougarjson.WorkerEphJson
}

func NewWorkerEphEvent(ctx context.Context, workerId data.WorkerId, workerEph *cougarjson.WorkerEphJson) *WorkerEphEvent {
	return &WorkerEphEvent{
		Ctx:       ctx,
		WorkerId:  workerId,
		WorkerEph: workerEph,
	}
}

func (e *WorkerEphEvent) GetName() string {
	return "WorkerEphEvent"
}

func (e *WorkerEphEvent) Process(ctx context.Context, ss *ServiceState) {
	ss.StagingWorkerEph(e.Ctx, e.WorkerId, e.WorkerEph)
}

// stagingWorkerEph: must call in runloop. Staging area is write by (individual) worker eph event(s), read by digestStagingWorkerEph (in batch)
func (ss *ServiceState) StagingWorkerEph(ctx context.Context, workerId data.WorkerId, workerEph *cougarjson.WorkerEphJson) {
	klogging.Verbose(ctx).With("workerId", workerId).
		With("hasEph", workerEph != nil).With("eph", workerEph).
		Log("stagingWorkerEph", "开始处理worker eph")

	defer func() {
		klogging.Verbose(ctx).With("workerId", workerId).
			With("hasEph", workerEph != nil).With("eph", workerEph).
			Log("stagingWorkerEph", "处理worker eph完成")
	}()

	ss.syncWorkerBatchManager.TryScheduleInternal(ctx, "StagingWorkerEph:"+string(workerId))

	if workerEph == nil {
		// delete
		// step 1: find out the workerFullId
		dict, ok := ss.EphWorkerStaging[workerId]
		if !ok {
			ke := kerror.Create("DeletedEphNotFound", "worker eph not found in staging area") // this should not happen
			panic(ke)
		}
		if len(dict) >= 2 {
			ke := kerror.Create("DeletedEphNotFound", "multiple worker eph in staging area") // this should not happen
			panic(ke)
		}
		var workerFullId data.WorkerFullId
		for k := range dict {
			workerFullId = data.WorkerFullId{
				WorkerId:  workerId,
				SessionId: k,
			}
			delete(dict, k)
		}
		// step 2: delete the workerEph from staging area
		delete(ss.EphWorkerStaging, workerId)
		ss.EphDirty[workerFullId] = common.Unit{}
		// klogging.Info(ctx).With("workerFullId", workerFullId.String()).
		// 	Log("stagingWorkerEph", "worker eph已删除")
		return
	}

	workerFullId := data.NewWorkerFullId(workerId, data.SessionId(workerEph.SessionId), data.StatefulType(workerEph.StatefulType))
	dict, ok := ss.EphWorkerStaging[workerId]
	if ok { // defensive coding, this is rarely happen (is it even possible?)
		// update
		dict[workerFullId.SessionId] = workerEph
		ss.EphDirty[workerFullId] = common.Unit{}
		return
	}

	// add
	ss.EphWorkerStaging[workerId] = map[data.SessionId]*cougarjson.WorkerEphJson{workerFullId.SessionId: workerEph}
	ss.EphDirty[workerFullId] = common.Unit{}
	// klogging.Info(ctx).With("workerFullId", workerFullId.String()).
	// 	Log("stagingWorkerEph", "worker eph已更新")
}

func (ss *ServiceState) batchAddToStagingWorkerEph(ctx context.Context, workers []*cougarjson.WorkerEphJson) {
	for _, workerEph := range workers {
		workerId := data.WorkerId(workerEph.WorkerId)
		sessionId := data.SessionId(workerEph.SessionId)
		ss.EphWorkerStaging[workerId] = map[data.SessionId]*cougarjson.WorkerEphJson{sessionId: workerEph}
		workerFullId := data.NewWorkerFullId(workerId, sessionId, data.StatefulType(workerEph.StatefulType))
		ss.EphDirty[workerFullId] = common.Unit{}
	}
}
