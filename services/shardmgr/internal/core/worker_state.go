package core

import (
	"context"

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
	// only updates those have dirty flag
	for workerFullId := range ss.EphDirty {
		workerEph := ss.EphWorkerStaging[workerFullId]
		workerState := ss.AllWorkers[workerFullId]
		if workerState == nil {
			if workerEph == nil {
				// nothing to do
			} else {
				// Worker Event: Eph node created, worker becomes online
				workerState = NewWorkerState(data.WorkerId(workerEph.WorkerId), data.SessionId(workerEph.SessionId), workerEph)
				ss.AllWorkers[workerFullId] = workerState
			}
		} else {
			if workerEph == nil {
				// Worker Event: Eph node lost, worker becomes offline
				workerState.onEphNodeLost(ctx)
			} else {
				// Worker Event: Eph node updated
				workerState.onEphNodeUpdate(ctx, workerEph)
			}
		}
	}
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
