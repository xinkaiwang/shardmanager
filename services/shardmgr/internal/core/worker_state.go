package core

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/xinkaiwang/shardmanager/libs/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/libs/unicorn/unicornjson"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
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
	parent             *ServiceState
	WorkerId           data.WorkerId
	SessionId          data.SessionId
	State              data.WorkerStateEnum
	EphNode            *cougarjson.WorkerEphJson
	ShutdownRequesting bool
	// ShutdownPermited       bool
	GracePeriodStartTimeMs int64

	Assignments map[data.ShardId]data.AssignmentId
	// Shards      map[data.ShardId]data.AssignmentId // 1 shard may have multiple replica/assignments, but those multiple assignments should never live on the same worker

	NotifyReason string
	SignalBox    *SignalBox
	WorkerInfo   *smgjson.WorkerInfoJson
	// WorkerReportedAssignments map[data.AssignmentId]common.Unit
	StatefulType data.StatefulType
}

func NewWorkerState(ss *ServiceState, workerId data.WorkerId, sessionId data.SessionId, statefulType data.StatefulType) *WorkerState {
	return &WorkerState{
		parent:             ss,
		WorkerId:           workerId,
		SessionId:          sessionId,
		State:              data.WS_Online_healthy,
		ShutdownRequesting: false,
		// ShutdownPermited:          false,
		Assignments: map[data.ShardId]data.AssignmentId{},
		// Shards:      make(map[data.ShardId]data.AssignmentId), // empty at first
		SignalBox:  NewSignalBox(),
		WorkerInfo: nil,
		// WorkerReportedAssignments: make(map[data.AssignmentId]common.Unit),
		StatefulType: statefulType,
	}
}

func NewWorkerStateFromJson(ctx context.Context, ss *ServiceState, workerStateJson *smgjson.WorkerStateJson) *WorkerState {
	workerState := &WorkerState{
		parent:    ss,
		WorkerId:  workerStateJson.WorkerId,
		SessionId: workerStateJson.SessionId,
		State:     workerStateJson.WorkerState,
		// Assignments:        make(map[data.AssignmentId]common.Unit),
		Assignments:        make(map[data.ShardId]data.AssignmentId), // empty at first
		ShutdownRequesting: common.BoolFromInt8(workerStateJson.RequestShutdown),
		SignalBox:          NewSignalBox(),
		WorkerInfo:         workerStateJson.WorkerInfo,
	}
	if common.BoolFromInt8(workerStateJson.Hat) {
		ss.hatTryGet(ctx, workerState.GetWorkerFullId())
	}
	if workerState.WorkerInfo == nil {
		workerState.WorkerInfo = smgjson.NewWorkerInfoJson()
	}
	workerState.StatefulType = data.StatefulTypeParseFromString(workerStateJson.StatefulType)
	for assignId, assignmentJson := range workerStateJson.Assignments {
		if assignmentJson.CurrentState == cougarjson.CAS_Dropped {
			continue // skip dropped assignments
		}
		assignmentId := data.AssignmentId(assignId)
		assignmentState := NewAssignmentState(assignmentId, assignmentJson.ShardId, assignmentJson.ReplicaIdx, workerState.GetWorkerFullId())
		assignmentState.TargetState = assignmentJson.TargetState
		assignmentState.CurrentConfirmedState = assignmentJson.CurrentState
		assignmentState.ShouldInPilot = common.BoolFromInt8(assignmentJson.InPilot)
		assignmentState.ShouldInRoutingTable = common.BoolFromInt8(assignmentJson.InRouting)
		workerState.AddAssignment(ctx, assignmentId, assignmentJson.ShardId)
		// workerState.Assignments[assignmentId] = common.Unit{}
		ss.AllAssignments[assignmentId] = assignmentState
	}
	return workerState
}

func (ws *WorkerState) AddAssignment(ctx context.Context, assignmentId data.AssignmentId, shardId data.ShardId) {
	if oldAssignId, ok := ws.Assignments[shardId]; ok {
		klogging.Fatal(ctx).With("workerId", ws.WorkerId).With("shardId", shardId).With("oldAssignId", oldAssignId).With("assignId", assignmentId).Log("AddAssignment", "shard already exists in worker state")
	}
	ws.Assignments[shardId] = assignmentId
}
func (ws *WorkerState) RemoveAssignment(ctx context.Context, shardId data.ShardId) {
	delete(ws.Assignments, shardId)
}

func (ws *WorkerState) Journal(ctx context.Context, reason string) {
	klogging.Info(ctx).With("workerId", ws.GetWorkerFullId().String()).With("workerState", ws.ToFullString()).With("state", ws.State).With("reason", reason).Log("WorkerJournal", "")
}

func (ws *WorkerState) ToWorkerStateJson(ctx context.Context, ss *ServiceState, updateReason string) *smgjson.WorkerStateJson {
	obj := &smgjson.WorkerStateJson{
		WorkerId:         ws.WorkerId,
		SessionId:        ws.SessionId,
		WorkerInfo:       ws.WorkerInfo,
		WorkerState:      ws.State,
		Assignments:      map[data.AssignmentId]*smgjson.AssignmentStateJson{},
		LastUpdateAtMs:   kcommon.GetWallTimeMs(),
		LastUpdateReason: updateReason,
		RequestShutdown:  common.Int8FromBool(ws.ShutdownRequesting),
		Hat:              common.Int8FromBool(ws.HasShutdownHat()),
		StatefulType:     string(ws.StatefulType),
	}
	for _, assignmentId := range ws.Assignments {
		assignState, ok := ss.AllAssignments[assignmentId]
		if !ok {
			klogging.Fatal(ctx).With("assignmentId", assignmentId).
				Log("ToWorkerStateJson", "assignment not found")
			continue
		}
		assignJson := smgjson.NewAssignmentStateJson(assignState.ShardId, assignState.ReplicaIdx)
		assignJson.TargetState = assignState.TargetState
		assignJson.CurrentState = assignState.CurrentConfirmedState
		assignJson.InPilot = common.Int8FromBool(assignState.ShouldInPilot)
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

func (ss *WorkerState) ToFullString() string {
	dict := make(map[string]interface{})
	dict["WorkerId"] = ss.WorkerId
	dict["SessionId"] = ss.SessionId
	dict["State"] = ss.State
	dict["ShutdownRequesting"] = ss.ShutdownRequesting
	dict["ShutdownPermited"] = ss.GetShutdownPermited()
	var assignments []string
	for assignId := range ss.Assignments {
		assignments = append(assignments, string(assignId))
	}
	dict["Assignments"] = assignments
	dict["StatefulType"] = ss.StatefulType
	dict["WorkerInfo"] = &ss.WorkerInfo
	data, err := json.Marshal(dict)
	if err != nil {
		ke := kerror.Wrap(err, "MarshalError", "failed to marshal WorkerState", false)
		panic(ke)
	}
	return string(data)
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
		ss.applyNewWorker(ctx, workerFullId, workerEph, "FirstDigestStagingWorkerEph")
	}
}

func (ss *ServiceState) applyNewWorker(ctx context.Context, workerFullId data.WorkerFullId, workerEph *cougarjson.WorkerEphJson, reason string) {
	// add to ss
	workerState := NewWorkerState(ss, workerFullId.WorkerId, workerFullId.SessionId, data.StatefulType(workerEph.StatefulType))
	workerState.WorkerInfo = smgjson.WorkerInfoFromWorkerEph(workerEph)
	workerState.applyNewEph(ctx, ss, workerEph, map[data.AssignmentId]*AssignmentState{})
	workerState.Journal(ctx, "NewWorkerFromEph")
	ss.AllWorkers[workerFullId] = workerState
	ss.FlushWorkerState(ctx, workerFullId, workerState, FS_Most, "NewWorkerFromEph")
	// add to current/future snapshot
	workerSnap := costfunc.NewWorkerSnap(workerState.GetWorkerFullId())
	workerSnap.Draining = workerState.ShutdownRequesting
	fn := func(snapshot *costfunc.Snapshot) {
		snapshot.AllWorkers.Set(workerFullId, workerSnap)
	}
	ss.ModifySnapshot(ctx, fn, reason)
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
				ss.applyNewWorker(ctx, workerFullId, workerEph, "digestStagingWorkerEph")
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

func (ws *WorkerState) signalAll(ctx context.Context, reason string) {
	currentBox := ws.SignalBox
	ws.SignalBox = NewSignalBox()
	currentBox.NotifyReason = reason
	currentBox.NofityId = kcommon.RandomString(ctx, 8)
	close(currentBox.NotifyCh)
	klogging.Info(ctx).With("workerId", ws.WorkerId).With("boxId", currentBox.BoxId).With("reason", reason).With("notifyId", currentBox.NofityId).Log("signalAll", "worker state changed")
}

func (ws *WorkerState) setWorkerState(ctx context.Context, newState data.WorkerStateEnum, reason string) {
	klogging.Info(ctx).With("workerId", ws.GetWorkerFullId().String()).With("oldState", ws.State).With("newState", newState).With("reason", reason).Log("setWorkerState", "worker state changed")
	if newState == data.WS_Online_shutdown_permit || newState == data.WS_Offline_draining_complete { // this worker draining complete, don't need a hat anymore
		ws.parent.hatReturn(ctx, ws.GetWorkerFullId())
	}
	ws.State = newState
}

// onEphNodeLost: must be called in runloop
func (ws *WorkerState) onEphNodeLost(ctx context.Context, ss *ServiceState) *DirtyFlag {
	ws.EphNode = nil
	var dirty DirtyFlag
	switch ws.State {
	case data.WS_Online_healthy:
		// Worker Event: Eph node lost, worker becomes offline
		ws.setWorkerState(ctx, data.WS_Offline_graceful_period, "onEphNodeLost")
		ws.GracePeriodStartTimeMs = kcommon.GetWallTimeMs()
		dirty.AddDirtyFlag("WS_Offline_graceful_period")
	case data.WS_Online_shutdown_req:
		// Worker Event: Eph node lost, worker becomes offline
		ws.setWorkerState(ctx, data.WS_Offline_graceful_period, "onEphNodeLost")
		ws.GracePeriodStartTimeMs = kcommon.GetWallTimeMs()
		dirty.AddDirtyFlag("WS_Offline_graceful_period")
	case data.WS_Online_shutdown_permit:
		// Worker Event: Eph node lost, worker becomes offline
		ws.setWorkerState(ctx, data.WS_Offline_draining_complete, "onEphNodeLost")
		ws.GracePeriodStartTimeMs = kcommon.GetWallTimeMs()
		dirty.AddDirtyFlag("WS_Offline_draining_complete")
	case data.WS_Offline_graceful_period:
		fallthrough
	case data.WS_Offline_draining_candidate:
		fallthrough
	case data.WS_Offline_draining_complete:
		// ws.setWorkerState(ctx, data.WS_Offline_dead, "onEphNodeLost")
		// dirty.AddDirtyFlag("WS_Offline_dead")
		fallthrough
	case data.WS_Offline_dead:
		// nothing to do
	}
	klogging.Info(ctx).With("workerId", ws.GetWorkerFullId().String()).With("sessionId", ws.SessionId).
		With("dirty", dirty.String()).
		With("state", ws.State).
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
	ws.EphNode = workerEph
	dirtyFlag := NewDirtyFlag()
	var passiveMoves []costfunc.PassiveMove
	switch ws.State {
	case data.WS_Online_healthy:
		fallthrough
	case data.WS_Online_shutdown_req:
		fallthrough
	case data.WS_Online_shutdown_permit:
		dict := ws.CollectCurrentAssignments(ss)
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
		for _, move := range passiveMoves {
			ss.ModifySnapshot(ctx, move.Apply, move.Signature())
		}
	}
	return dirtyFlag
}

func (ws *WorkerState) IsOnline() bool {
	return ws.EphNode != nil
}

// CollectCurrentAssignments: collect current assignments from AllAssignments, fatal if not found
func (ws *WorkerState) CollectCurrentAssignments(ss *ServiceState) map[data.AssignmentId]*AssignmentState {
	dict := make(map[data.AssignmentId]*AssignmentState)
	for _, assignmentId := range ws.Assignments {
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
	if !ws.WorkerInfo.Equals(newWorkerInfo) {
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
		if assignState.CurrentConfirmedState == cougarjson.CAS_Ready || assignState.CurrentConfirmedState == cougarjson.CAS_Dropping {
			oldState := assignState.CurrentConfirmedState
			assignState.CurrentConfirmedState = cougarjson.CAS_Dropped
			passiveMove := NewPasMoveRemoveAssignment(assignState.AssignmentId, assignState.ShardId, assignState.ReplicaIdx, ws.GetWorkerFullId())
			moves = append(moves, passiveMove)
			dirtyFlag.AddDirtyFlag("unassign:" + string(assignState.AssignmentId) + ":" + string(oldState) + ":" + string(assignState.CurrentConfirmedState))
		}
	}
	// request shutdown
	if !ws.ShutdownRequesting {
		if common.BoolFromInt8(workerEph.ReqShutDown) {
			ws.ShutdownRequesting = true
			dirtyFlag.AddDirtyFlag("ReqShutdown")
			ws.setWorkerState(ctx, data.WS_Online_shutdown_req, "ReqShutdown")
			hat := ss.hatTryGet(ctx, ws.GetWorkerFullId())
			if hat {
				dirtyFlag.AddDirtyFlag("hat")
			}
			if ws.shouldWorkerIncludeInSnapshot() {
				drain := ws.HasShutdownHat()
				passiveMove := costfunc.NewPasMoveWorkerSnapUpdate(ws.GetWorkerFullId(), func(ws *costfunc.WorkerSnap) {
					ws.Draining = drain
					ws.NotTarget = true // this worker is not available to move any shard into it
				}, "WorkerStateUpdate")
				moves = append(moves, passiveMove)
			} else {
				passiveMove := costfunc.NewPasMoveWorkerSnapAddRemove(ws.GetWorkerFullId(), nil, "WorkerStateAddRemove")
				moves = append(moves, passiveMove)
			}
		}
	}

	return
}

func (ws *WorkerState) checkWorkerOnTimeout(ctx context.Context, ss *ServiceState) (needsDelete bool) {
	currentState := ws.State
	dirty := NewDirtyFlag()
	var passiveMoves []costfunc.PassiveMove
	switch ws.State {
	case data.WS_Online_healthy:
		// nothing to do
	case data.WS_Online_shutdown_req:
		// nothing to do
		if len(ws.Assignments) == 0 {
			ws.setWorkerState(ctx, data.WS_Online_shutdown_permit, "checkWorkerOnTimeout")
			dirty.AddDirtyFlag("WS_Online_shutdown_permit")
		}
	case data.WS_Online_shutdown_permit:
		// nothing to do
	case data.WS_Offline_graceful_period:
		if kcommon.GetWallTimeMs() < ws.GracePeriodStartTimeMs+int64(ss.ServiceConfig.FaultToleranceConfig.GracePeriodSecBeforeDrain)*1000 { // not expired yet
			break
		}
		// Worker Event: Grace period expired, worker becomes offline
		ws.setWorkerState(ctx, data.WS_Offline_draining_candidate, "checkWorkerOnTimeout")
		ws.GracePeriodStartTimeMs = kcommon.GetWallTimeMs() // until grace period expired
		dirty.AddDirtyFlag("WS_Offline_draining_candidate")
		if !ws.HasShutdownHat() {
			hat := ss.hatTryGet(ctx, ws.GetWorkerFullId())
			if hat {
				dirty.AddDirtyFlag("hat")
				passiveMove := costfunc.NewPasMoveWorkerSnapUpdate(ws.GetWorkerFullId(), func(ws *costfunc.WorkerSnap) {
					ws.Draining = true
				}, "drainingHat")
				passiveMoves = append(passiveMoves, passiveMove)
			}
		}
		fallthrough
	case data.WS_Offline_draining_candidate:
		reason := "checkWorkerOnTimeout"
		// clean purge = this worker don't have any assignments
		if len(ws.Assignments) > 0 {
			// dirty purge = this worker has assignments, we will wait for the grace period until force (dirty) purge
			if kcommon.GetWallTimeMs() < ws.GracePeriodStartTimeMs+int64(ss.ServiceConfig.FaultToleranceConfig.GracePeriodSecBeforeDirtyPurge)*1000 {
				break
			}
			// Grace period expired, DirtyPurge this worker
			// ws.setWorkerState(ctx, data.WS_Offline_dead, "checkWorkerOnTimeout")
			// remove all assignments
			for _, assignId := range ws.Assignments {
				assignState, ok := ss.AllAssignments[assignId]
				if !ok {
					continue
				}
				passiveMove := NewPasMoveRemoveAssignment(assignState.AssignmentId, assignState.ShardId, assignState.ReplicaIdx, ws.GetWorkerFullId())
				passiveMoves = append(passiveMoves, passiveMove)
				passiveMove.ApplyToSs(ctx, ss)
			}
			reason = "checkWorkerOnTimeout_DirtyPurge"
			// passiveMove := costfunc.NewPasMoveWorkerSnapAddRemove(ws.GetWorkerFullId(), nil, "WS_Offline_dead")
			// passiveMoves = append(passiveMoves, passiveMove)
			// dirty.AddDirtyFlag("WS_Offline_dead")
		}
		// Worker Event: Draining complete, worker becomes offline
		ws.setWorkerState(ctx, data.WS_Offline_draining_complete, reason)
		ws.GracePeriodStartTimeMs = kcommon.GetWallTimeMs() // DeleteGracePeriodSec = 20 seconds, then clean it up
		dirty.AddDirtyFlag("WS_Offline_draining_complete")
		fallthrough
	case data.WS_Offline_draining_complete:
		if kcommon.GetWallTimeMs() < ws.GracePeriodStartTimeMs+20*1000 {
			break
		}
		// Worker Event: Grace period expired, worker becomes offline
		ws.setWorkerState(ctx, data.WS_Offline_dead, "checkWorkerOnTimeout")
		passiveMove := costfunc.NewPasMoveWorkerSnapAddRemove(ws.GetWorkerFullId(), nil, "WS_Offline_dead")
		passiveMoves = append(passiveMoves, passiveMove)
		dirty.AddDirtyFlag("WS_Offline_dead")
		needsDelete = true
	}
	if dirty.IsDirty() {
		klogging.Info(ctx).With("workerId", ws.WorkerId).
			With("dirty", dirty.String()).
			With("currentState", currentState).
			With("newState", ws.State).
			Log("checkWorkerOnTimeout", "worker 定时任务完成")
	}
	if !dirty.IsDirty() {
		return
	}

	if needsDelete {
		ss.FlushWorkerState(ctx, ws.GetWorkerFullId(), nil, FS_Most, dirty.String())
	} else {
		ss.FlushWorkerState(ctx, ws.GetWorkerFullId(), ws, FS_Most, dirty.String())
	}
	for _, move := range passiveMoves {
		ss.ModifySnapshot(ctx, move.Apply, move.Signature())
	}

	return
}

// checkWorkerForHat: see whether this worker needs shutdown hat.
func (ws *WorkerState) checkWorkerForHat(ctx context.Context) bool { // return isDirty
	// check if this worker needs shutdown hat
	if ws.State == data.WS_Online_shutdown_req || ws.State == data.WS_Offline_draining_candidate {
		if ws.HasShutdownHat() {
			return false // already has shutdown hat, no need to get again
		}
		hat := ws.parent.hatTryGet(ctx, ws.GetWorkerFullId())
		return hat
	}
	return false
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
	_, ok := ws.parent.ShutdownHat[ws.GetWorkerFullId()]
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
	for _, assignmentId := range ws.Assignments {
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
		string(ws.SessionId),
		ws.WorkerInfo.AddressPort,
		updateReason)

	// 将WorkerState中的Assignments转换为AssignmentJson列表
	entry.Assignments = make([]*unicornjson.AssignmentJson, 0, len(ws.Assignments))

	// 遍历每个分配任务，创建相应的AssignmentJson
	for _, assignmentId := range ws.Assignments {
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
