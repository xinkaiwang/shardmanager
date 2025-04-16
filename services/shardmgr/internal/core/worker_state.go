package core

import (
	"context"
	"strconv"
	"strings"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
	"github.com/xinkaiwang/shardmanager/services/unicorn/unicornjson"
)

type SignalBox struct {
	BoxId        string
	NotifyReason string
	NofityId     string
	NotifyCh     chan common.Unit // notify when WorkerState changes
}

func NewSignalBox() *SignalBox {
	return &SignalBox{
		BoxId:        kcommon.RandomString(context.Background(), 8),
		NotifyCh:     make(chan common.Unit, 1),
		NotifyReason: "",
	}
}

type WorkerState struct {
	WorkerId           data.WorkerId
	SessionId          data.SessionId
	State              data.WorkerStateEnum
	ShutdownRequesting bool
	// ShutdownPermited       bool
	GracePeriodStartTimeMs int64

	Assignments map[data.AssignmentId]common.Unit

	NotifyReason              string
	SignalBox                 *SignalBox
	WorkerInfo                smgjson.WorkerInfoJson
	WorkerReportedAssignments map[data.AssignmentId]common.Unit
	StatefulType              data.StatefulType
}

func NewWorkerState(workerId data.WorkerId, sessionId data.SessionId, statefulType data.StatefulType) *WorkerState {
	return &WorkerState{
		WorkerId:           workerId,
		SessionId:          sessionId,
		State:              data.WS_Online_healthy,
		ShutdownRequesting: false,
		// ShutdownPermited:          false,
		Assignments:               make(map[data.AssignmentId]common.Unit),
		SignalBox:                 NewSignalBox(),
		WorkerInfo:                smgjson.WorkerInfoJson{},
		WorkerReportedAssignments: make(map[data.AssignmentId]common.Unit),
		StatefulType:              statefulType,
	}
}

func NewWorkerStateFromJson(workerStateJson *smgjson.WorkerStateJson) *WorkerState {
	workerState := &WorkerState{
		WorkerId:     workerStateJson.WorkerId,
		SessionId:    workerStateJson.SessionId,
		State:        workerStateJson.WorkerState,
		Assignments:  make(map[data.AssignmentId]common.Unit),
		SignalBox:    NewSignalBox(),
		WorkerInfo:   smgjson.WorkerInfoJson{},
		StatefulType: workerStateJson.StatefulType,
	}
	return workerState
}

func (ws *WorkerState) ToWorkerStateJson(ctx context.Context, ss *ServiceState, updateReason string) *smgjson.WorkerStateJson {
	obj := &smgjson.WorkerStateJson{
		WorkerId:         ws.WorkerId,
		SessionId:        ws.SessionId,
		WorkerState:      ws.State,
		Assignments:      map[data.AssignmentId]*smgjson.AssignmentStateJson{},
		LastUpdateAtMs:   kcommon.GetWallTimeMs(),
		LastUpdateReason: updateReason,
		RequestShutdown:  common.Int8FromBool(ws.ShutdownRequesting),
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

func (ws *WorkerState) GetWorkerFullId() data.WorkerFullId {
	return data.NewWorkerFullId(ws.WorkerId, ws.SessionId, ws.StatefulType)
}

func (ws *WorkerState) GetShutdownRequesting() bool {
	return ws.ShutdownRequesting
}
func (ws *WorkerState) GetShutdownPermited() bool {
	return ws.State == data.WS_Online_shutdown_permit ||
		ws.State == data.WS_Offline_draining_complete ||
		ws.State == data.WS_Offline_dead ||
		ws.State == data.WS_Deleted
}

func (ss *ServiceState) firstDigestStagingWorkerEph(ctx context.Context) {
	workers := map[data.WorkerFullId]*cougarjson.WorkerEphJson{}
	for workerId, dict := range ss.EphWorkerStaging {
		for sessionId, workerEph := range dict {
			workerFullId := data.NewWorkerFullId(workerId, sessionId, data.StatefulType(workerEph.StatefulType))
			workers[workerFullId] = workerEph
		}
	}
	// compare with current workerStates and decide add/update/delete
	for workerFullId, workerState := range ss.AllWorkers {
		if workerEph, ok := workers[workerFullId]; ok {
			// update
			delete(workers, workerFullId)
			workerState.onEphNodeUpdate(ctx, ss, workerEph)
		} else {
			// delete
			workerState.onEphNodeLost(ctx, ss)
		}
	}
	// remaining workers are new
	for workerFullId, workerEph := range workers {
		// add
		workerState := NewWorkerState(data.WorkerId(workerEph.WorkerId), data.SessionId(workerEph.SessionId), data.StatefulType(workerEph.StatefulType))
		workerState.updateWorkerByEph(ctx, workerEph)
		ss.AllWorkers[workerFullId] = workerState
		ss.FlushWorkerState(ctx, workerFullId, workerState, "NewWorker")
	}
}

// digestStagingWorkerEph: must be called in runloop.
// syncs from eph staging to worker state, should batch as much as possible
func (ss *ServiceState) digestStagingWorkerEph(ctx context.Context) {
	// klogging.Info(ctx).With("dirtyCount", len(ss.EphDirty)).
	// 	Log("syncEphStagingToWorkerState", "开始同步worker eph到worker state")

	// only updates those have dirty flag
	for workerFullId := range ss.EphDirty {
		workerEph := ss.EphWorkerStaging[workerFullId.WorkerId][workerFullId.SessionId]
		workerState := ss.AllWorkers[workerFullId]

		klogging.Info(ctx).With("workerFullId", workerFullId.String()).
			With("hasEph", workerEph != nil).
			With("hasState", workerState != nil).
			Log("syncEphStagingToWorkerState", "处理worker")

		if workerState == nil {
			if workerEph == nil {
				// nothing to do
			} else {
				// Worker Event: Eph node created, worker becomes online
				workerState = NewWorkerState(data.WorkerId(workerEph.WorkerId), data.SessionId(workerEph.SessionId), data.StatefulType(workerEph.StatefulType))
				workerState.updateWorkerByEph(ctx, workerEph)
				ss.AllWorkers[workerFullId] = workerState
				ss.FlushWorkerState(ctx, workerFullId, workerState, "NewWorker")
			}
		} else {
			if workerEph == nil {
				// Worker Event: Eph node lost, worker becomes offline

				workerState.onEphNodeLost(ctx, ss)
			} else {
				// Worker Event: Eph node updated

				workerState.onEphNodeUpdate(ctx, ss, workerEph)
			}
		}
	}

	ss.EphDirty = make(map[data.WorkerFullId]common.Unit)
}

func (ss *ServiceState) ApplyPassiveMove(ctx context.Context, move costfunc.PassiveMove, mode costfunc.ApplyMode) {
	newCurrent := move.Apply(ss.SnapshotCurrent, mode).Freeze()
	newFuture := move.Apply(ss.SnapshotFuture, mode).Freeze()
	ss.SnapshotCurrent = newCurrent
	ss.SnapshotFuture = newFuture
	if ss.SolverGroup != nil {
		ss.SolverGroup.OnSnapshot(ss.SnapshotFuture)
	}
}

func (ws *WorkerState) signalAll(ctx context.Context, reason string) {
	currentBox := ws.SignalBox
	ws.SignalBox = NewSignalBox()
	currentBox.NotifyReason = reason
	currentBox.NofityId = kcommon.RandomString(ctx, 8)
	close(currentBox.NotifyCh)
	klogging.Info(ctx).With("workerId", ws.WorkerId).With("boxId", currentBox.BoxId).With("reason", reason).With("notifyId", currentBox.NofityId).Log("signalAll", "worker state changed")
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
		ws.State = data.WS_Offline_draining_complete
		ws.GracePeriodStartTimeMs = kcommon.GetWallTimeMs()
		dirty.AddDirtyFlag("WorkerState")
	case data.WS_Offline_graceful_period:
		fallthrough
	case data.WS_Offline_draining_candidate:
		fallthrough
	case data.WS_Offline_draining_hat:
		fallthrough
	case data.WS_Offline_draining_complete:
		// this should never happen
		klogging.Fatal(ctx).With("workerId", ws.WorkerId).With("currentState", ws.State).
			Log("onEphNodeLost", "worker eph already lost")
	case data.WS_Offline_dead:
		// nothing to do
	}
	klogging.Info(ctx).With("workerId", ws.WorkerId).
		With("dirty", dirty.String()).
		Log("onEphNodeLostDone", "worker eph lost")
	if dirty.IsDirty() {
		reason := dirty.String()
		ss.FlushWorkerState(ctx, ws.GetWorkerFullId(), ws, reason)
		ws.signalAll(ctx, "onEphNodeLost:"+reason)
	}
}

// onEphNodeUpdate: must be called in runloop
func (ws *WorkerState) onEphNodeUpdate(ctx context.Context, ss *ServiceState, workerEph *cougarjson.WorkerEphJson) {
	dirty := NewDirtyFlag()
	dict := ws.collectCurrentAssignments(ss)
	updateCount := ws.applyNewEph(ctx, ss, workerEph, dict)
	if updateCount > 0 {
		dirty.AddDirtyFlag("asign" + strconv.Itoa(updateCount))
	}
	switch ws.State {
	case data.WS_Online_healthy:
		fallthrough
	case data.WS_Online_shutdown_req:
		fallthrough
	case data.WS_Online_shutdown_hat:
		fallthrough
	case data.WS_Online_shutdown_permit:
		dirty.AddDirtyFlags(ws.updateWorkerByEph(ctx, workerEph))
	case data.WS_Offline_graceful_period:
		fallthrough
	case data.WS_Offline_draining_candidate:
		fallthrough
	case data.WS_Offline_draining_hat:
		fallthrough
	case data.WS_Offline_draining_complete:
		fallthrough
	case data.WS_Offline_dead:
		// this should never happen
		klogging.Fatal(ctx).With("workerId", ws.WorkerId).With("currentState", ws.State).
			Log("onEphNodeUpdate", "worker eph already lost")
	}
	klogging.Info(ctx).With("workerId", ws.WorkerId).
		With("dirty", dirty.String()).
		Log("onEphNodeUpdateDone", "workerEph updated finished")
	if dirty.IsDirty() {
		reason := dirty.String()
		ss.FlushWorkerState(ctx, ws.GetWorkerFullId(), ws, reason)
		ws.signalAll(ctx, "onEphNodeUpdate:"+reason)
	}
}

func (ws *WorkerState) collectCurrentAssignments(ss *ServiceState) map[data.AssignmentId]*AssignmentState {
	dict := make(map[data.AssignmentId]*AssignmentState)
	for assignmentId := range ws.Assignments {
		assignState, ok := ss.AllAssignments[assignmentId]
		if !ok {
			klogging.Fatal(context.Background()).With("workerId", ws.WorkerId).
				With("assignmentId", assignmentId).
				Log("collectCurrentAssignments", "assignment not found")
			continue
		}
		dict[assignmentId] = assignState
	}
	return dict
}

func (ws *WorkerState) applyNewEph(ctx context.Context, ss *ServiceState, workerEph *cougarjson.WorkerEphJson, dict map[data.AssignmentId]*AssignmentState) int {
	updateCount := 0
	for _, assignmentJson := range workerEph.Assignments {
		dirty := false
		assignmentId := data.AssignmentId(assignmentJson.AssignmentId)
		assignState, ok := dict[assignmentId]
		if !ok {
			continue
		}
		delete(dict, assignmentId)
		if assignState.CurrentConfirmedState != assignmentJson.State {
			assignState.CurrentConfirmedState = assignmentJson.State
			dirty = true
		}
		if dirty {
			updateCount++
		}
	}
	// all remaining assignments are dropped from eph
	for _, assignState := range dict {
		if assignState.CurrentConfirmedState != cougarjson.CAS_Dropped {
			assignState.CurrentConfirmedState = cougarjson.CAS_Dropped
			updateCount++
		}
	}
	return updateCount
}

func (ws *WorkerState) checkWorkerOnTimeout(ctx context.Context, ss *ServiceState) (needsDelete bool) {
	currentState := ws.State
	dirty := NewDirtyFlag()
	switch ws.State {
	case data.WS_Online_healthy:
		// nothing to do
	case data.WS_Online_shutdown_req:
		// try to get a hat
		if !ss.hatTryGet(ctx, ws.GetWorkerFullId()) {
			break
		}
		// Worker Event: Hat got, worker becomes offline
		ws.State = data.WS_Online_shutdown_hat
		dirty.AddDirtyFlag("WorkerState")
		fallthrough
	case data.WS_Online_shutdown_hat:
		// try to see if the worker is already empty (no assignments)
		if len(ws.Assignments) > 0 {
			break
		}
		// Worker Event: Draining complete, worker becomes offline
		ws.State = data.WS_Online_shutdown_permit
		ss.hatReturn(ctx, ws.GetWorkerFullId())
		dirty.AddDirtyFlag("WorkerState")
		fallthrough
	case data.WS_Online_shutdown_permit:
		// nothing to do
	case data.WS_Offline_graceful_period:
		if kcommon.GetWallTimeMs() < ws.GracePeriodStartTimeMs+int64(ss.ServiceConfig.WorkerConfig.OfflineGracePeriodSec)*1000 { // not expired yet
			break
		}
		// Worker Event: Grace period expired, worker becomes offline
		ws.State = data.WS_Offline_draining_candidate
		dirty.AddDirtyFlag("WorkerState")
		fallthrough
	case data.WS_Offline_draining_candidate:
		// try to get the draining hat
		if !ss.hatTryGet(ctx, ws.GetWorkerFullId()) {
			break
		}
		// Worker Event: Hat applied, worker becomes offline
		ws.State = data.WS_Offline_draining_hat
		dirty.AddDirtyFlag("WorkerState")
		fallthrough
	case data.WS_Offline_draining_hat:
		// try to see if the worker is already empty (no assignments)
		if len(ws.Assignments) > 0 {
			break
		}
		// Worker Event: Draining complete, worker becomes offline
		ws.State = data.WS_Offline_draining_complete
		ws.GracePeriodStartTimeMs = kcommon.GetWallTimeMs()
		ss.hatReturn(ctx, ws.GetWorkerFullId())
		dirty.AddDirtyFlag("WorkerState")
		fallthrough
	case data.WS_Offline_draining_complete:
		if kcommon.GetWallTimeMs() < ws.GracePeriodStartTimeMs+20*1000 { // DeleteGracePeriodSec = 20 seconds, then clean it up
			break
		}
		// Worker Event: Grace period expired, worker becomes offline
		ws.State = data.WS_Offline_dead
		dirty.AddDirtyFlag("WorkerState")
		needsDelete = true
	}
	if dirty.IsDirty() {
		klogging.Info(ctx).With("workerId", ws.WorkerId).
			With("dirty", dirty.String()).
			With("currentState", currentState).
			With("newState", ws.State).
			Log("checkWorkerOnTimeout", "worker 定时任务完成")
	}
	if dirty.IsDirty() {
		if needsDelete {
			ss.FlushWorkerState(ctx, ws.GetWorkerFullId(), nil, dirty.String())
			// ss.StoreProvider.StoreWorkerState(ws.GetWorkerFullId(ss), nil)
		} else {
			ss.FlushWorkerState(ctx, ws.GetWorkerFullId(), ws, dirty.String())
			// ss.StoreProvider.StoreWorkerState(ws.GetWorkerFullId(ss), ws.ToWorkerStateJson(ctx, ss, dirty.String()))
		}
	}
	return
}

// return true if dirty
func (ws *WorkerState) updateWorkerByEph(ctx context.Context, workerEph *cougarjson.WorkerEphJson) *DirtyFlag {
	dirty := NewDirtyFlag()
	// WorkerInfo
	newWorkerInfo := smgjson.WorkerInfoFromWorkerEph(workerEph)
	if !ws.WorkerInfo.Equals(&newWorkerInfo) {
		ws.WorkerInfo = newWorkerInfo
		dirty.AddDirtyFlag("WorkerInfo")
	}
	// Assignments
	newReportedAssign := make(map[data.AssignmentId]common.Unit)
	for _, assignment := range workerEph.Assignments {
		newReportedAssign[data.AssignmentId(assignment.AssignmentId)] = common.Unit{}
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

type DirtyFlag struct {
	flags []string
}

func NewDirtyFlag() *DirtyFlag {
	return &DirtyFlag{}
}

func (df *DirtyFlag) AddDirtyFlag(flag ...string) *DirtyFlag {
	df.flags = append(df.flags, flag...)
	return df
}

func (df *DirtyFlag) AddDirtyFlags(another *DirtyFlag) *DirtyFlag {
	df.flags = append(df.flags, another.flags...)
	return df
}

func (df *DirtyFlag) IsDirty() bool {
	return len(df.flags) > 0
}

func (df *DirtyFlag) String() string {
	return strings.Join(df.flags, ",")
}

func (ws *WorkerState) HasShutdownHat() bool {
	return ws.State == data.WS_Online_shutdown_hat || ws.State == data.WS_Offline_draining_hat
}

func (ws *WorkerState) ToPilotNode(ctx context.Context, ss *ServiceState, updateReason string) *cougarjson.PilotNodeJson {
	// 创建PilotNodeJson对象
	pilotNode := cougarjson.NewPilotNodeJson(
		string(ws.WorkerId),
		string(ws.SessionId),
		updateReason,
	)
	if ws.GetShutdownPermited() {
		pilotNode.ShutdownPermited = 1
	}

	// 将WorkerState中的Assignments转换为PilotAssignmentJson列表
	pilotNode.Assignments = make([]*cougarjson.PilotAssignmentJson, 0, len(ws.Assignments))

	// 遍历每个分配任务，创建相应的PilotAssignmentJson
	for assignmentId := range ws.Assignments {
		assign := ss.AllAssignments[assignmentId]
		if assign == nil {
			klogging.Fatal(ctx).
				With("workerId", ws.WorkerId).
				With("sessionId", ws.SessionId).
				With("assignmentId", assignmentId).
				Log("ToPilotNode", "assignment not found")
			continue
		}
		if !assign.ShouldInPilot {
			continue
		}
		// 创建PilotAssignmentJson
		pilotAssignment := cougarjson.NewPilotAssignmentJson(
			string(assign.ShardId),
			int(assign.ReplicaIdx),
			string(assignmentId),
			assign.TargetState,
		)

		// 添加到PilotNode
		pilotNode.Assignments = append(pilotNode.Assignments, pilotAssignment)
	}

	return pilotNode
}

// func (ws *WorkerState) Ase2PilotState(targetState smgjson.AssignmentStateEnum) cougarjson.PilotAssignmentState {
// 	switch targetState {
// 	case smgjson.ASE_Unknown:
// 		return cougarjson.PAS_Unknown
// 	case smgjson.ASE_Healthy:
// 		return cougarjson.PAS_Active
// 	case smgjson.ASE_Dropped:
// 		return cougarjson.PAS_Completed
// 	default:
// 		return cougarjson.PAS_Active
// 	}
// }

func (ws *WorkerState) ToRoutingEntry(ctx context.Context, ss *ServiceState, updateReason string) *unicornjson.WorkerEntryJson {
	// 创建WorkerEntryJson对象
	entry := unicornjson.NewWorkerEntryJson(
		string(ws.WorkerId),
		ws.WorkerInfo.AddressPort,
		updateReason)

	// 将WorkerState中的Assignments转换为AssignmentJson列表
	entry.Assignments = make([]*unicornjson.AssignmentJson, 0, len(ws.Assignments))

	// 遍历每个分配任务，创建相应的AssignmentJson
	for assignmentId := range ws.Assignments {
		assign := ss.AllAssignments[assignmentId]
		if assign == nil {
			klogging.Fatal(ctx).
				With("workerId", ws.WorkerId).
				With("sessionId", ws.SessionId).
				With("assignmentId", assignmentId).
				Log("ToRoutingEntry", "assignment not found")
			continue
		}
		// 创建AssignmentJson
		assignment := unicornjson.NewAssignmentJson(
			string(assign.ShardId),
			int(assign.ReplicaIdx),
			string(assignmentId),
		)

		// 添加到WorkerEntry
		entry.Assignments = append(entry.Assignments, assignment)
	}

	return entry
}
