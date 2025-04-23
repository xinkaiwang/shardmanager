package core

import (
	"context"
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
	// parent             *ServiceState
	WorkerId           data.WorkerId
	SessionId          data.SessionId
	State              data.WorkerStateEnum
	ShutdownRequesting bool
	// ShutdownPermited       bool
	GracePeriodStartTimeMs int64

	Assignments map[data.AssignmentId]common.Unit

	NotifyReason string
	SignalBox    *SignalBox
	WorkerInfo   smgjson.WorkerInfoJson
	// WorkerReportedAssignments map[data.AssignmentId]common.Unit
	StatefulType data.StatefulType
}

func NewWorkerState(workerId data.WorkerId, sessionId data.SessionId, statefulType data.StatefulType) *WorkerState {
	return &WorkerState{
		WorkerId:           workerId,
		SessionId:          sessionId,
		State:              data.WS_Online_healthy,
		ShutdownRequesting: false,
		// ShutdownPermited:          false,
		Assignments: make(map[data.AssignmentId]common.Unit),
		SignalBox:   NewSignalBox(),
		WorkerInfo:  smgjson.WorkerInfoJson{},
		// WorkerReportedAssignments: make(map[data.AssignmentId]common.Unit),
		StatefulType: statefulType,
	}
}

func NewWorkerStateFromJson(ss *ServiceState, workerStateJson *smgjson.WorkerStateJson) *WorkerState {
	workerState := &WorkerState{
		WorkerId:    workerStateJson.WorkerId,
		SessionId:   workerStateJson.SessionId,
		State:       workerStateJson.WorkerState,
		Assignments: make(map[data.AssignmentId]common.Unit),
		SignalBox:   NewSignalBox(),
		WorkerInfo:  smgjson.WorkerInfoJson{},
	}
	workerState.StatefulType = data.StatefulTypeParseFromString(workerStateJson.StatefulType)
	for assignId, assignmentJson := range workerStateJson.Assignments {
		assignmentId := data.AssignmentId(assignId)
		assignmentState := NewAssignmentState(assignmentId, assignmentJson.ShardId, assignmentJson.ReplicaIdx, workerState.GetWorkerFullId())
		assignmentState.TargetState = assignmentJson.TargetState
		assignmentState.CurrentConfirmedState = assignmentJson.TargetState
		assignmentState.ShouldInRoutingTable = common.BoolFromInt8(assignmentJson.InRouting)
		workerState.Assignments[assignmentId] = common.Unit{}
		ss.AllAssignments[assignmentId] = assignmentState
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
		StatefulType:     string(ws.StatefulType),
	}
	for assignmentId := range ws.Assignments {
		assignState, ok := ss.AllAssignments[assignmentId]
		if !ok {
			klogging.Fatal(ctx).With("assignmentId", assignmentId).
				Log("ToWorkerStateJson", "assignment not found")
			continue
		}
		assignJson := smgjson.NewAssignmentStateJson(assignState.ShardId, assignState.ReplicaIdx)
		assignJson.TargetState = assignState.TargetState
		assignJson.CurrentState = assignState.CurrentConfirmedState
		assignJson.InRouting = common.Int8FromBool(assignState.ShouldInRoutingTable)
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
		workerState.applyNewEph(ctx, ss, workerEph, map[data.AssignmentId]*AssignmentState{})
		ss.AllWorkers[workerFullId] = workerState
		ss.FlushWorkerState(ctx, workerFullId, workerState, FS_Most, "NewWorker")
	}
}

// digestStagingWorkerEph: must be called in runloop.
// syncs from eph staging to worker state, should batch as much as possible
func (ss *ServiceState) digestStagingWorkerEph(ctx context.Context) {
	// klogging.Info(ctx).With("dirtyCount", len(ss.EphDirty)).
	// 	Log("digestStagingWorkerEph", "开始同步worker eph到worker state")
	dirtyFlag := NewDirtyFlag()
	// only updates those have dirty flag
	for workerFullId := range ss.EphDirty {
		workerEph := ss.EphWorkerStaging[workerFullId.WorkerId][workerFullId.SessionId]
		workerState := ss.AllWorkers[workerFullId]

		klogging.Info(ctx).With("workerFullId", workerFullId.String()).
			With("hasEph", workerEph != nil).
			With("hasState", workerState != nil).
			Log("digestStagingWorkerEph", "处理worker")

		if workerState == nil {
			if workerEph == nil {
				// nothing to do
			} else {
				// Worker Event: Eph node created, worker becomes online
				workerState = NewWorkerState(data.WorkerId(workerEph.WorkerId), data.SessionId(workerEph.SessionId), data.StatefulType(workerEph.StatefulType))
				workerState.applyNewEph(ctx, ss, workerEph, map[data.AssignmentId]*AssignmentState{})
				ss.AllWorkers[workerFullId] = workerState
				klogging.Info(ctx).With("workerFullId", workerFullId.String()).Log("digestStagingWorkerEph", "新建 worker state")
				ss.FlushWorkerState(ctx, workerFullId, workerState, FS_Most, "NewWorker")
				// add this new worker in current/future snapshot
				workerSnap := costfunc.NewWorkerSnap(workerState.GetWorkerFullId())
				workerSnap.Draining = workerState.ShutdownRequesting
				ss.snapshotOperationManager.TrySchedule(ctx, func(snapshot *costfunc.Snapshot) {
					snapshot.AllWorkers.Set(workerFullId, workerSnap)
				}, "AddWorkerToSnapshot")
				// newCurrent := ss.SnapshotCurrent.Clone()
				// newCurrent.AllWorkers.Set(workerFullId, workerSnap)
				// ss.SnapshotCurrent = newCurrent.Freeze()
				// newFuture := ss.SnapshotFuture.Clone()
				// newFuture.AllWorkers.Set(workerFullId, workerSnap)
				// ss.SnapshotFuture = newFuture.Freeze()

				// dirtyFlag
				dirtyFlag.AddDirtyFlag("NewWorker:" + workerFullId.String())
			}
		} else {
			if workerEph == nil {
				// Worker Event: Eph node lost, worker becomes offline
				dirtyFlag.AddDirtyFlags(workerState.onEphNodeLost(ctx, ss))
			} else {
				// Worker Event: Eph node updated
				dirtyFlag.AddDirtyFlags(workerState.onEphNodeUpdate(ctx, ss, workerEph))
			}
		}
	}

	ss.EphDirty = make(map[data.WorkerFullId]common.Unit)
}

func (ss *ServiceState) ApplyPassiveMove(ctx context.Context, moves []costfunc.PassiveMove, mode costfunc.ApplyMode) {
	newCurrent := ss.SnapshotCurrent.Clone()
	newFuture := ss.SnapshotFuture.Clone()
	var reason []string
	for _, move := range moves {
		reason = append(reason, move.Signature())
		newCurrent = move.Apply(newCurrent, mode)
		newFuture = move.Apply(newFuture, mode)
	}
	ss.SnapshotCurrent = newCurrent.Freeze()
	ss.SnapshotFuture = newFuture.Freeze()
	if ss.SolverGroup != nil {
		ss.SolverGroup.OnSnapshot(ctx, ss.SnapshotFuture, strings.Join(reason, ","))
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
func (ws *WorkerState) onEphNodeLost(ctx context.Context, ss *ServiceState) *DirtyFlag {
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
	case data.WS_Online_shutdown_permit:
		// Worker Event: Eph node lost, worker becomes offline
		ws.State = data.WS_Offline_draining_complete
		ws.GracePeriodStartTimeMs = kcommon.GetWallTimeMs()
		dirty.AddDirtyFlag("WorkerState")
	case data.WS_Offline_graceful_period:
		fallthrough
	case data.WS_Offline_draining_candidate:
		fallthrough
	case data.WS_Offline_draining_complete:
		ws.State = data.WS_Offline_dead
		dirty.AddDirtyFlag("WorkerState")
	case data.WS_Offline_dead:
		// nothing to do
	}
	klogging.Info(ctx).With("workerId", ws.WorkerId).
		With("dirty", dirty.String()).
		Log("onEphNodeLostDone", "worker eph lost")
	if dirty.IsDirty() {
		reason := dirty.String()
		ss.FlushWorkerState(ctx, ws.GetWorkerFullId(), ws, FS_Most, reason)
		ws.signalAll(ctx, "onEphNodeLost:"+reason)
	}
	return &dirty
}

// onEphNodeUpdate: must be called in runloop
func (ws *WorkerState) onEphNodeUpdate(ctx context.Context, ss *ServiceState, workerEph *cougarjson.WorkerEphJson) *DirtyFlag {
	dirtyFlag := NewDirtyFlag()
	var passiveMoves []costfunc.PassiveMove
	switch ws.State {
	case data.WS_Online_healthy:
		fallthrough
	case data.WS_Online_shutdown_req:
		fallthrough
	case data.WS_Online_shutdown_permit:
		dict := ws.collectCurrentAssignments(ss)
		dirty, moves := ws.applyNewEph(ctx, ss, workerEph, dict)
		dirtyFlag.AddDirtyFlags(dirty)
		passiveMoves = append(passiveMoves, moves...)
	case data.WS_Offline_graceful_period:
		fallthrough
	case data.WS_Offline_draining_candidate:
		fallthrough
	case data.WS_Offline_draining_complete:
		fallthrough
	case data.WS_Offline_dead:
		// this should never happen
		klogging.Fatal(ctx).With("workerId", ws.WorkerId).With("currentState", ws.State).
			Log("onEphNodeUpdate", "worker eph already lost")
	}
	klogging.Info(ctx).With("workerId", ws.WorkerId).
		With("dirty", dirtyFlag.String()).
		Log("onEphNodeUpdateDone", "workerEph updated finished")
	if dirtyFlag.IsDirty() {
		reason := dirtyFlag.String()
		ss.FlushWorkerState(ctx, ws.GetWorkerFullId(), ws, FS_Most, reason)
		ws.signalAll(ctx, "onEphNodeUpdate:"+reason)
	}
	if len(passiveMoves) > 0 {
		ss.ApplyPassiveMove(ctx, passiveMoves, costfunc.AM_Strict)
	}
	return dirtyFlag
}

// collectCurrentAssignments: collect current assignments from AllAssignments, fatal if not found
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

func (ws *WorkerState) applyNewEph(ctx context.Context, ss *ServiceState, workerEph *cougarjson.WorkerEphJson, dict map[data.AssignmentId]*AssignmentState) (dirtyFlag *DirtyFlag, moves []costfunc.PassiveMove) {
	dirtyFlag = NewDirtyFlag()
	// WorkerInfo
	newWorkerInfo := smgjson.WorkerInfoFromWorkerEph(workerEph)
	if !ws.WorkerInfo.Equals(&newWorkerInfo) {
		ws.WorkerInfo = newWorkerInfo
		dirtyFlag.AddDirtyFlag("WorkerInfo")
	}
	// full compare A) dict (current) vs B) workerEph.Assignments (new)
	for _, assignmentJson := range workerEph.Assignments {
		assignmentId := data.AssignmentId(assignmentJson.AssignmentId)
		assignState, ok := dict[assignmentId]
		if !ok {
			// case 1: need add?
			// how can this possible? assignment should added by minion before pilot/eph node have it.
			// in this case we ignore it.
			continue
		}
		delete(dict, assignmentId)
		// case 2: need update
		if assignState.CurrentConfirmedState != assignmentJson.State {
			assignState.CurrentConfirmedState = assignmentJson.State
			dirtyFlag.AddDirtyFlag("updateAssign:" + string(assignState.AssignmentId) + ":" + string(assignState.CurrentConfirmedState))
		}
	}
	// case 3: need delete
	for _, assignState := range dict {
		if assignState.CurrentConfirmedState != cougarjson.CAS_Dropped {
			assignState.CurrentConfirmedState = cougarjson.CAS_Dropped
			dirtyFlag.AddDirtyFlag("unassign:" + string(assignState.AssignmentId) + ":" + string(assignState.CurrentConfirmedState))
		}
	}
	// request shutdown
	if !ws.ShutdownRequesting {
		if common.BoolFromInt8(workerEph.ReqShutDown) {
			ws.ShutdownRequesting = true
			dirtyFlag.AddDirtyFlag("ReqShutdown")
			ws.State = data.WS_Online_shutdown_req
			ss.hatTryGet(ctx, ws.GetWorkerFullId())
			passiveMove := costfunc.NewWorkerStateChange(ws.GetWorkerFullId(), ws.State, ws.HasShutdownHat(ss))
			moves = append(moves, passiveMove)
		}
	}

	return
}

func (ws *WorkerState) checkWorkerOnTimeout(ctx context.Context, ss *ServiceState) (needsDelete bool) {
	currentState := ws.State
	dirty := NewDirtyFlag()
	switch ws.State {
	case data.WS_Online_healthy:
		// nothing to do
	case data.WS_Online_shutdown_req:
		// nothing to do
		if len(ws.Assignments) == 0 {
			ws.State = data.WS_Online_shutdown_permit
			dirty.AddDirtyFlag("WorkerState:" + string(ws.State))
		}
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
		// try to see if the worker is already empty (no assignments)
		if len(ws.Assignments) > 0 {
			break
		}
		// Worker Event: Draining complete, worker becomes offline
		ws.State = data.WS_Offline_draining_complete
		ws.GracePeriodStartTimeMs = kcommon.GetWallTimeMs()
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
			ss.FlushWorkerState(ctx, ws.GetWorkerFullId(), nil, FS_Most, dirty.String())
		} else {
			ss.FlushWorkerState(ctx, ws.GetWorkerFullId(), ws, FS_Most, dirty.String())
		}
	}
	return
}

// // return true if dirty
// func (ws *WorkerState) updateWorkerByEph(ctx context.Context, workerEph *cougarjson.WorkerEphJson) *DirtyFlag {
// 	dirty := NewDirtyFlag()
// 	// // WorkerInfo
// 	// newWorkerInfo := smgjson.WorkerInfoFromWorkerEph(workerEph)
// 	// if !ws.WorkerInfo.Equals(&newWorkerInfo) {
// 	// 	ws.WorkerInfo = newWorkerInfo
// 	// 	dirty.AddDirtyFlag("WorkerInfo")
// 	// }
// 	// // Assignments
// 	// newReportedAssign := make(map[data.AssignmentId]common.Unit)
// 	// for _, assignment := range workerEph.Assignments {
// 	// 	newReportedAssign[data.AssignmentId(assignment.AssignmentId)] = common.Unit{}
// 	// }
// 	// if !assignMapEquals(ws.WorkerReportedAssignments, newReportedAssign) {
// 	// 	ws.WorkerReportedAssignments = newReportedAssign
// 	// 	dirty.AddDirtyFlag("Assign")
// 	// }
// 	// // request shutdown
// 	// if !ws.ShutdownRequesting {
// 	// 	if common.BoolFromInt8(workerEph.ReqShutDown) {
// 	// 		ws.ShutdownRequesting = true
// 	// 		dirty.AddDirtyFlag("ReqShutdown")
// 	// 		ws.State = data.WS_Online_shutdown_req
// 	// 		dirty.AddDirtyFlag("WorkerState")
// 	// 	}
// 	// }
// 	return dirty
// }

// func assignMapEquals(map1 map[data.AssignmentId]common.Unit, map2 map[data.AssignmentId]common.Unit) bool {
// 	if len(map1) != len(map2) {
// 		return false
// 	}
// 	for key := range map1 {
// 		if _, ok := map2[key]; !ok {
// 			return false
// 		}
// 	}
// 	return true
// }

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

func (ws *WorkerState) HasShutdownHat(ss *ServiceState) bool {
	_, ok := ss.ShutdownHat[ws.GetWorkerFullId()]
	return ok
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
		if !assign.ShouldInRoutingTable {
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
