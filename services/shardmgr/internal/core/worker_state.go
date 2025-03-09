package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

type WorkerState struct {
	WorkerId           data.WorkerId
	SessionId          data.SessionId
	State              data.WorkerStateEnum
	ShutdownRequesting bool
	ShutdownPermited   bool

	Assignments map[data.AssignmentId]common.Unit

	NotifyMajorCh chan struct{} // notify when WorkerStateEnum changes
	NotifyMinorCh chan struct{} // notify when WorkerStateEnum keeps the same but have some minor updates
	WorkerInfo    smgjson.WorkerInfoJson
	WorkerEph     *cougarjson.WorkerEphJson
}

func NewWorkerState(workerId data.WorkerId, sessionId data.SessionId, workerEph *cougarjson.WorkerEphJson) *WorkerState {
	return &WorkerState{
		WorkerId:           workerId,
		SessionId:          sessionId,
		State:              data.WS_Online_healthy,
		ShutdownRequesting: false,
		ShutdownPermited:   false,
		Assignments:        make(map[data.AssignmentId]common.Unit),
		NotifyMajorCh:      make(chan struct{}, 1),
		NotifyMinorCh:      make(chan struct{}, 1),
		WorkerInfo:         smgjson.WorkerInfoFromWorkerEph(workerEph),
	}
}

func NewWorkerStateFromJson(workerStateJson *smgjson.WorkerStateJson) *WorkerState {
	workerState := &WorkerState{
		WorkerId:      workerStateJson.WorkerId,
		SessionId:     workerStateJson.SessionId,
		State:         workerStateJson.WorkerState,
		NotifyMajorCh: make(chan struct{}, 1),
		NotifyMinorCh: make(chan struct{}, 1),
		WorkerInfo:    smgjson.WorkerInfoJson{},
	}
	return workerState
}

func (ws *WorkerState) GetWorkerFullId(ss *ServiceState) data.WorkerFullId {
	return data.NewWorkerFullId(ws.WorkerId, ws.SessionId, ss.IsStateInMemory())
}

// syncEphStagingToWorkerState: must be called in runloop.
// syncs from eph staging to worker state, should batch as much as possible
func (ss *ServiceState) syncEphStagingToWorkerState(ctx context.Context) {
	klogging.Info(ctx).With("dirtyCount", len(ss.EphDirty)).
		Log("syncEphStagingToWorkerState", "开始同步worker eph到worker state")

	// only updates those have dirty flag
	for workerFullId := range ss.EphDirty {
		workerEph := ss.EphWorkerStaging[workerFullId]
		workerState := ss.AllWorkers[workerFullId]

		klogging.Info(ctx).With("workerFullId", workerFullId.String()).
			With("hasEph", workerEph != nil).
			With("hasState", workerState != nil).
			Log("syncEphStagingToWorkerState", "处理worker")

		if workerState == nil {
			if workerEph == nil {
				// nothing to do
				klogging.Info(ctx).With("workerFullId", workerFullId.String()).
					Log("syncEphStagingToWorkerState", "无需操作：worker eph和state都不存在")
			} else {
				// Worker Event: Eph node created, worker becomes online
				klogging.Info(ctx).With("workerFullId", workerFullId.String()).
					Log("syncEphStagingToWorkerState", "正在创建新的worker state")

				workerState = NewWorkerState(data.WorkerId(workerEph.WorkerId), data.SessionId(workerEph.SessionId), workerEph)
				ss.AllWorkers[workerFullId] = workerState

				klogging.Info(ctx).With("workerFullId", workerFullId.String()).
					With("workerId", workerEph.WorkerId).
					With("sessionId", workerEph.SessionId).
					Log("syncEphStagingToWorkerState", "worker state创建完成")
			}
		} else {
			if workerEph == nil {
				// Worker Event: Eph node lost, worker becomes offline
				klogging.Info(ctx).With("workerFullId", workerFullId.String()).
					With("oldState", workerState.State).
					Log("syncEphStagingToWorkerState", "worker eph丢失，更新worker state")

				workerState.onEphNodeLost(ctx)

				klogging.Info(ctx).With("workerFullId", workerFullId.String()).
					With("newState", workerState.State).
					Log("syncEphStagingToWorkerState", "worker state已更新为离线状态")
			} else {
				// Worker Event: Eph node updated
				klogging.Info(ctx).With("workerFullId", workerFullId.String()).
					Log("syncEphStagingToWorkerState", "更新worker state")

				workerState.onEphNodeUpdate(ctx, workerEph)

				klogging.Info(ctx).With("workerFullId", workerFullId.String()).
					With("state", workerState.State).
					Log("syncEphStagingToWorkerState", "worker state已更新")
			}
		}
	}

	klogging.Info(ctx).With("processingCount", len(ss.EphDirty)).
		With("totalWorkers", len(ss.AllWorkers)).
		Log("syncEphStagingToWorkerState", "worker eph同步到worker state完成")

	ss.EphDirty = make(map[data.WorkerFullId]common.Unit)
}

// onEphNodeLost: must be called in runloop
func (ws *WorkerState) onEphNodeLost(ctx context.Context) {
	switch ws.State {
	case data.WS_Online_healthy:
		// Worker Event: Eph node lost, worker becomes offline
		ws.State = data.WS_Offline_graceful_period
	case data.WS_Online_shutdown_req:
		// Worker Event: Eph node lost, worker becomes offline
		ws.State = data.WS_Offline_graceful_period
	case data.WS_Online_shutdown_hat:
		// Worker Event: Eph node lost, worker becomes offline
		ws.State = data.WS_Offline_draining_hat
	case data.WS_Online_shutdown_permit:
		// Worker Event: Eph node lost, worker becomes offline
		ws.State = data.WS_Offline_dead
	case data.WS_Offline_graceful_period:
		// nothing to do
	case data.WS_Offline_draining_candidate:
		// nothing to do
	case data.WS_Offline_draining_hat:
		// nothing to do
	case data.WS_Offline_draining_complete:
		// nothing to do
	case data.WS_Offline_dead:
		// nothing to do
	}
}

// onEphNodeUpdate: must be called in runloop
func (ws *WorkerState) onEphNodeUpdate(ctx context.Context, workerEph *cougarjson.WorkerEphJson) {
	switch ws.State {
	case data.WS_Online_healthy:
		// nothing to do
	case data.WS_Online_shutdown_req:
		// nothing to do
	case data.WS_Online_shutdown_hat:
		// nothing to do
	case data.WS_Online_shutdown_permit:
		// nothing to do
	case data.WS_Offline_graceful_period:
		// nothing to do
	case data.WS_Offline_draining_candidate:
		// nothing to do
	case data.WS_Offline_draining_hat:
		// nothing to do
	case data.WS_Offline_draining_complete:
		// nothing to do
	case data.WS_Offline_dead:
		// nothing to do
	}
}
