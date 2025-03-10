package core

import (
	"context"
	"strings"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

type WorkerState struct {
	WorkerId               data.WorkerId
	SessionId              data.SessionId
	State                  data.WorkerStateEnum
	ShutdownRequesting     bool
	ShutdownPermited       bool
	GracePeriodStartTimeMs int64

	Assignments map[data.AssignmentId]common.Unit

	NotifyReason              string
	NotifyCh                  chan struct{} // notify when WorkerState changes
	WorkerInfo                smgjson.WorkerInfoJson
	WorkerEph                 *cougarjson.WorkerEphJson
	WorkerReportedAssignments map[data.AssignmentId]common.Unit
}

func NewWorkerState(workerId data.WorkerId, sessionId data.SessionId) *WorkerState {
	return &WorkerState{
		WorkerId:                  workerId,
		SessionId:                 sessionId,
		State:                     data.WS_Online_healthy,
		ShutdownRequesting:        false,
		ShutdownPermited:          false,
		Assignments:               make(map[data.AssignmentId]common.Unit),
		NotifyCh:                  make(chan struct{}, 1),
		WorkerInfo:                smgjson.WorkerInfoJson{},
		WorkerReportedAssignments: make(map[data.AssignmentId]common.Unit),
	}
}

func NewWorkerStateFromJson(workerStateJson *smgjson.WorkerStateJson) *WorkerState {
	workerState := &WorkerState{
		WorkerId:    workerStateJson.WorkerId,
		SessionId:   workerStateJson.SessionId,
		State:       workerStateJson.WorkerState,
		Assignments: make(map[data.AssignmentId]common.Unit),
		NotifyCh:    make(chan struct{}, 1),
		WorkerInfo:  smgjson.WorkerInfoJson{},
	}
	return workerState
}

func (ws *WorkerState) ToWorkerStateJson(ctx context.Context, ss *ServiceState) *smgjson.WorkerStateJson {
	obj := &smgjson.WorkerStateJson{
		WorkerId:    ws.WorkerId,
		SessionId:   ws.SessionId,
		WorkerState: ws.State,
		Assignments: map[data.AssignmentId]*smgjson.AssignmentStateJson{},
	}
	for assignmentId := range ws.Assignments {
		assignState, ok := ss.AllAssignments[assignmentId]
		if !ok {
			klogging.Fatal(ctx).With("assignmentId", assignmentId).
				Log("ToWorkerStateJson", "assignment not found")
			continue
		}
		assignJson := &smgjson.AssignmentStateJson{
			ShardId:    assignState.ShardId,
			ReplicaIdx: assignState.ReplicaIdx,
		}
		obj.Assignments[assignmentId] = assignJson
	}
	return obj
}

func (ws *WorkerState) GetState() data.WorkerStateEnum {
	return ws.State
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

				workerState = NewWorkerState(data.WorkerId(workerEph.WorkerId), data.SessionId(workerEph.SessionId))
				workerState.updateWorkerByEph(ctx, workerEph)
				ss.AllWorkers[workerFullId] = workerState
				ss.StoreProvider.StoreWorkerState(workerFullId, workerState.ToWorkerStateJson(ctx, ss).SetUpdateReason("worker_create"))

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

				workerState.onEphNodeLost(ctx, ss)
				ss.StoreProvider.StoreWorkerState(workerFullId, workerState.ToWorkerStateJson(ctx, ss).SetUpdateReason("worker_eph_lost"))

				klogging.Info(ctx).With("workerFullId", workerFullId.String()).
					With("newState", workerState.State).
					Log("syncEphStagingToWorkerState", "worker state已更新为离线状态")
			} else {
				// Worker Event: Eph node updated
				klogging.Info(ctx).With("workerFullId", workerFullId.String()).
					Log("syncEphStagingToWorkerState", "更新worker state")

				workerState.onEphNodeUpdate(ctx, ss, workerEph)
				ss.StoreProvider.StoreWorkerState(workerFullId, workerState.ToWorkerStateJson(ctx, ss).SetUpdateReason("worker_eph_update"))

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

func (ws *WorkerState) signalAll(reason string) {
	ws.NotifyReason = reason
	close(ws.NotifyCh)
	ws.NotifyCh = make(chan struct{}, 1)
}

// onEphNodeLost: must be called in runloop
func (ws *WorkerState) onEphNodeLost(ctx context.Context, ss *ServiceState) {
	var dirty DirtyFlag
	switch ws.State {
	case data.WS_Online_healthy:
		// Worker Event: Eph node lost, worker becomes offline
		ws.State = data.WS_Offline_graceful_period
		ws.GracePeriodStartTimeMs = kcommon.GetWallTimeMs()
		dirty.AddDirtyFlag("WorkerState")
	case data.WS_Online_shutdown_req:
		// Worker Event: Eph node lost, worker becomes offline
		ws.State = data.WS_Offline_graceful_period
		ws.GracePeriodStartTimeMs = kcommon.GetWallTimeMs()
		dirty.AddDirtyFlag("WorkerState")
	case data.WS_Online_shutdown_hat:
		// Worker Event: Eph node lost, worker becomes offline
		ws.State = data.WS_Offline_draining_hat
		dirty.AddDirtyFlag("WorkerState")
	case data.WS_Online_shutdown_permit:
		// Worker Event: Eph node lost, worker becomes offline
		ws.State = data.WS_Offline_dead
		ws.GracePeriodStartTimeMs = kcommon.GetWallTimeMs()
		dirty.AddDirtyFlag("WorkerState")
	case data.WS_Offline_graceful_period:
	case data.WS_Offline_draining_candidate:
	case data.WS_Offline_draining_hat:
	case data.WS_Offline_draining_complete:
		// this should never happen
		klogging.Fatal(ctx).With("workerId", ws.WorkerId).With("currentState", ws.State).
			Log("onEphNodeLost", "worker eph already lost")
	case data.WS_Offline_dead:
		// nothing to do
	}
	if dirty.IsDirty() {
		reason := dirty.String()
		ss.StoreProvider.StoreWorkerState(ws.GetWorkerFullId(ss), ws.ToWorkerStateJson(ctx, ss).SetUpdateReason(reason))
		ws.signalAll(reason)
	}
}

// onEphNodeUpdate: must be called in runloop
func (ws *WorkerState) onEphNodeUpdate(ctx context.Context, ss *ServiceState, workerEph *cougarjson.WorkerEphJson) {
	var dirty DirtyFlag
	switch ws.State {
	case data.WS_Online_healthy:
	case data.WS_Online_shutdown_req:
	case data.WS_Online_shutdown_hat:
	case data.WS_Online_shutdown_permit:
		dirty.AddDirtyFlag(ws.updateWorkerByEph(ctx, workerEph)...)
	case data.WS_Offline_graceful_period:
	case data.WS_Offline_draining_candidate:
	case data.WS_Offline_draining_hat:
	case data.WS_Offline_draining_complete:
	case data.WS_Offline_dead:
		// this should never happen
		klogging.Fatal(ctx).With("workerId", ws.WorkerId).With("currentState", ws.State).
			Log("onEphNodeUpdate", "worker eph already lost")
	}
	if dirty.IsDirty() {
		reason := dirty.String()
		ss.StoreProvider.StoreWorkerState(ws.GetWorkerFullId(ss), ws.ToWorkerStateJson(ctx, ss).SetUpdateReason(reason))
		ws.signalAll(reason)
	}
}

func (ws *WorkerState) checkWorkerForTimeout(ctx context.Context, ss *ServiceState) (needsDelete bool) {
	var dirty DirtyFlag
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
		if ws.GracePeriodStartTimeMs+int64(ss.ServiceConfig.WorkerConfig.OfflineGracePeriodSec)*1000 <= kcommon.GetWallTimeMs() { // expired
			// Worker Event: Grace period expired, worker becomes offline
			ws.State = data.WS_Offline_draining_candidate
			dirty.AddDirtyFlag("WorkerState")
		}
	case data.WS_Offline_draining_candidate:
		// nothing to do
	case data.WS_Offline_draining_hat:
		// nothing to do
	case data.WS_Offline_draining_complete:
		// nothing to do
	case data.WS_Offline_dead:
		if ws.GracePeriodStartTimeMs+20*1000 > kcommon.GetWallTimeMs() { // worker in this state for 20 seconds, then clean it up
			// Worker Event: Grace period expired, worker becomes offline
			ws.State = data.WS_Deleted
			needsDelete = true
			dirty.AddDirtyFlag("WorkerState")
		}
		if dirty.IsDirty() {
			ss.StoreProvider.StoreWorkerState(ws.GetWorkerFullId(ss), ws.ToWorkerStateJson(ctx, ss).SetUpdateReason(dirty.String()))
		}
	}
	return
}

// return true if dirty
func (ws *WorkerState) updateWorkerByEph(ctx context.Context, workerEph *cougarjson.WorkerEphJson) DirtyFlag {
	var dirty DirtyFlag
	// WorkerInfo
	newWorkerInfo := smgjson.WorkerInfoFromWorkerEph(workerEph)
	if !ws.WorkerInfo.Equals(&newWorkerInfo) {
		ws.WorkerInfo = newWorkerInfo
		dirty.AddDirtyFlag("WorkerInfo")
	}
	// Assignments
	newReportedAssign := make(map[data.AssignmentId]common.Unit)
	for _, assignment := range workerEph.Assignments {
		newReportedAssign[data.AssignmentId(assignment.AsginmentId)] = common.Unit{}
	}
	if !assignMapEquals(ws.WorkerReportedAssignments, newReportedAssign) {
		ws.WorkerReportedAssignments = newReportedAssign
		dirty.AddDirtyFlag("Assign")
	}
	// request shutdown
	if !ws.ShutdownRequesting {
		if common.BoolFromInt8(workerEph.ReqShutDown) {
			ws.ShutdownRequesting = true
			dirty.AddDirtyFlag("ReqShutdown")
			ws.State = data.WS_Online_shutdown_req
			dirty.AddDirtyFlag("WorkerState")
		}
	}
	return dirty
}

func assignMapEquals(map1 map[data.AssignmentId]common.Unit, map2 map[data.AssignmentId]common.Unit) bool {
	if len(map1) != len(map2) {
		return false
	}
	for key := range map1 {
		if _, ok := map2[key]; !ok {
			return false
		}
	}
	return true
}

type DirtyFlag []string

func (df DirtyFlag) AddDirtyFlag(flag ...string) DirtyFlag {
	return append(df, flag...)
}

func (df DirtyFlag) IsDirty() bool {
	return len(df) > 0
}

func (df DirtyFlag) String() string {
	return strings.Join(df, ",")
}
