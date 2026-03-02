package core

import (
	"context"
	"log/slog"

	"github.com/xinkaiwang/shardmanager/libs/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/etcdprov"
)

type WorkerEphWatcher struct {
	ss      *ServiceState
	ch      chan etcdprov.EtcdKvItem
	stop    chan struct{} // used to stop the watcher
	stopped chan struct{} // used to notify when the watcher has stopped
}

func NewWorkerEphWatcher(ctx context.Context, ss *ServiceState, currentWorkerEphRevision etcdprov.EtcdRevision) *WorkerEphWatcher {
	watcher := &WorkerEphWatcher{
		ss:      ss,
		stop:    make(chan struct{}),
		stopped: make(chan struct{}),
	}
	watcher.ch = etcdprov.GetCurrentEtcdProvider(ctx).WatchByPrefix(ctx, ss.PathManager.GetWorkerEphPathPrefix(), currentWorkerEphRevision)
	go watcher.Run(ctx) // wew = worker eph watcher
	return watcher
}

func (w *WorkerEphWatcher) Run(ctx context.Context) {
	slog.InfoContext(ctx, "Started",
		slog.String("event", "WorkerEphWatcher"))
	stop := false
	for !stop {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "CtxDone.Exit",
				slog.String("event", "WorkerEphWatcher"))
			stop = true
		case kvItem := <-w.ch:
			if kvItem.Value == "" {
				// this is a delete event
				// str := kvItem.Key[len(w.ss.PathManager.GetWorkerEphPathPrefix()):] // exclude prefix '/smg/eph/'
				workerId := w.workerIdFromPath(kvItem.Key)
				slog.InfoContext(ctx, "观察到worker eph已删除",
					slog.String("event", "WorkerEphWatcher"),
					slog.Any("workerId", workerId),
					slog.Any("path", kvItem.Key))
				w.ss.PostEvent(NewWorkerEphEvent(ctx, workerId, nil))
				// klogging.Info(ctx).With("workerId", workerId).
				// 	Log("WorkerEphWatcher", "worker eph event posted")
				continue
			}
			// this is a add or update event
			slog.InfoContext(ctx, "观察到worker eph已更新",
				slog.String("event", "WorkerEphWatcher"),
				slog.Any("workerFullId", kvItem.Key))
			// str := kvItem.Key[len(w.ss.PathManager.GetWorkerEphPathPrefix()):] // exclude prefix '/smg/eph/'
			workerId := w.workerIdFromPath(kvItem.Key)
			workerEph := cougarjson.WorkerEphJsonFromJson(kvItem.Value)
			w.ss.PostEvent(NewWorkerEphEvent(ctx, workerId, workerEph))
			// klogging.Info(ctx).With("workerId", workerId).With("eph", workerEph).
			// 	Log("WorkerEphWatcher", "worker eph event posted")
		case <-w.stop:
			slog.InfoContext(ctx, "stop signal received",
				slog.String("event", "WorkerEphWatcher"))
			stop = true
		}
	}
	close(w.stopped)
}

func (w *WorkerEphWatcher) Stop() {
	// stop the watcher
	close(w.stop)
}

func (w *WorkerEphWatcher) StopAndWaitForExit() {
	// stop the watcher and wait for it to exit
	w.Stop()
	<-w.stopped // wait for the run thread to exit
	slog.InfoContext(context.Background(), "stopped",
		slog.String("event", "WorkerEphWatcher"))
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
	createTimeMs int64 // time when the event was created
	Ctx          context.Context
	WorkerId     data.WorkerId
	WorkerEph    *cougarjson.WorkerEphJson
}

func NewWorkerEphEvent(ctx context.Context, workerId data.WorkerId, workerEph *cougarjson.WorkerEphJson) *WorkerEphEvent {
	return &WorkerEphEvent{
		Ctx:          ctx,
		WorkerId:     workerId,
		WorkerEph:    workerEph,
		createTimeMs: kcommon.GetWallTimeMs(),
	}
}

func (e *WorkerEphEvent) GetCreateTimeMs() int64 {
	return e.createTimeMs
}
func (e *WorkerEphEvent) GetName() string {
	return "WorkerEphEvent"
}

func (e *WorkerEphEvent) Process(ctx context.Context, ss *ServiceState) {
	ss.StagingWorkerEph(e.Ctx, e.WorkerId, e.WorkerEph)
}

// stagingWorkerEph: must call in runloop. Staging area is write by (individual) worker eph event(s), read by digestStagingWorkerEph (in batch)
func (ss *ServiceState) StagingWorkerEph(ctx context.Context, workerId data.WorkerId, workerEph *cougarjson.WorkerEphJson) {
	slog.DebugContext(ctx, "开始处理worker eph",
		slog.String("event", "stagingWorkerEph"),
		slog.Any("workerId", workerId),
		slog.Any("hasEph", workerEph != nil),
		slog.Any("eph", workerEph))

	defer func() {
		slog.DebugContext(ctx, "处理worker eph完成",
			slog.String("event", "stagingWorkerEph"),
			slog.Any("workerId", workerId),
			slog.Any("hasEph", workerEph != nil),
			slog.Any("eph", workerEph))
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
