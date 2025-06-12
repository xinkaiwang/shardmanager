package core

import (
	"context"
	"encoding/json"

	"github.com/xinkaiwang/shardmanager/libs/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/libs/xklib/krunloop"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/shadow"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/solver"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

// ServiceState implements the CriticalResource interface
type ServiceState struct {
	Name        string
	runloop     *krunloop.RunLoop[*ServiceState]
	PathManager *config.PathManager
	// ServiceInfo     *ServiceInfo
	ServiceConfig   *config.ServiceConfig
	storeProvider   shadow.StoreProvider
	pilotProvider   shadow.PilotProvider
	routingProvider shadow.RoutingProvider
	actionProvider  shadow.ActionProvider
	ShadowState     shadow.InitListener
	SolverGroup     solver.SnapshotListener

	// Note: all the following fields are not thread-safe, one should never access them outside the runloop.
	AllShards      map[data.ShardId]*ShardState
	AllWorkers     map[data.WorkerFullId]*WorkerState
	AllAssignments map[data.AssignmentId]*AssignmentState
	AllMoves       map[data.ProposalId]*ActionMinion // in-flight moves

	DynamicThreshold *DynamicThreshold
	ProposalQueue    *ProposalQueue
	AcceptedCount    int                // mostly for testing/visibilty purpose
	SnapshotCurrent  *costfunc.Snapshot // current means current state
	SnapshotFuture   *costfunc.Snapshot // future = current + in_flight_moves (this is expected future, assume all moves goes well. most solver explore should be based on this.)

	// staging area: worker eph
	EphDirty         map[data.WorkerFullId]common.Unit
	EphWorkerStaging map[data.WorkerId]map[data.SessionId]*cougarjson.WorkerEphJson
	ShutdownHat      map[data.WorkerFullId]common.Unit // those worker with hat means they are in shutdown process
	// staging area: shard plan
	stagingShardPlan []*smgjson.ShardLineJson

	ShardPlanWatcher     *ShardPlanWatcher
	WorkerEphWatcher     *WorkerEphWatcher
	ServiceConfigWatcher *ServiceConfigWatcher

	syncWorkerBatchManager        *BatchManager // enqueue when any worker eph changed, dequeue= trigger ss.syncEphStagingToWorkerState()
	reCreateSnapshotBatchManager  *BatchManager // enqueue when any workerState/shardState add/remove/etc. dequeue=trigger snapshot recreate
	syncShardsBatchManager        *BatchManager // enqueue when 1) shard plan new/changed, 2) shard config changed, etc. dequeue=trigger ss.syncShardPlan()
	broadcastSnapshotBatchManager *BatchManager // dequeue=trigger snapshot broadcast
	// snapshotOperationManager      *SnapshotOperationManager // enqueue when any snapshot operation need to apply, dequeue=apply all operations to snapshot as batch, then broadcast snapshot

	// for metrics collection use only
	MetricsValues MetricsValues // all metrics values are atomic.Int64, so they are thread-safe

	LastHourlyCheckMs int64 // last snapshot jurnal ms, used for debug log
}

func NewServiceState(ctx context.Context, name string) *ServiceState {
	ss := &ServiceState{
		Name:             name,
		AllShards:        make(map[data.ShardId]*ShardState),
		AllWorkers:       make(map[data.WorkerFullId]*WorkerState),
		AllAssignments:   make(map[data.AssignmentId]*AssignmentState),
		AllMoves:         make(map[data.ProposalId]*ActionMinion),
		EphDirty:         make(map[data.WorkerFullId]common.Unit),
		EphWorkerStaging: make(map[data.WorkerId]map[data.SessionId]*cougarjson.WorkerEphJson),
		ShutdownHat:      make(map[data.WorkerFullId]common.Unit),
	}
	ss.PathManager = config.NewPathManager()
	ss.ProposalQueue = NewProposalQueue(ss, 20)
	ss.pilotProvider = shadow.NewPilotStore(ss.PathManager)
	ss.routingProvider = shadow.NewDefaultRoutingProvider(ss.PathManager)
	ss.actionProvider = shadow.NewDefaultActionProvider(ss.PathManager)
	ss.runloop = krunloop.NewRunLoop(ctx, ss, "ss")
	ss.runloop.InitTimeSeries(ctx, "AcceptEvent", "GetStateEvent", "Housekeep30sEvent", "Housekeep5sEvent", "Housekeep1sEvent", "WorkerEphEvent", "syncWorkerEphBatch", "reCreateSnapshotBatch", "syncShardPlanBatch", "boardcastSnapshotBatch")
	ss.syncWorkerBatchManager = NewBatchManager(ss, 10, "syncWorkerEphBatch", func(ctx context.Context, ss *ServiceState) {
		ss.digestStagingWorkerEph(ctx)
	})
	ss.reCreateSnapshotBatchManager = NewBatchManager(ss, 10, "reCreateSnapshotBatch", func(ctx context.Context, ss *ServiceState) {
		ss.ReCreateSnapshot(ctx, "reCreateSnapshotBatch")
	})
	ss.syncShardsBatchManager = NewBatchManager(ss, 10, "syncShardPlanBatch", func(ctx context.Context, ss *ServiceState) {
		dirty := ss.digestStagingShardPlan(ctx)
		if dirty {
			ss.ReCreateSnapshot(ctx, "syncShardPlanBatch")
		}
	})
	ss.broadcastSnapshotBatchManager = NewBatchManager(ss, 3, "broadcastSnapshotBatch", func(ctx context.Context, ss *ServiceState) {
		ss.broadcastSnapshot(ctx, "broadcastSnapshotBatch")
	})
	// ss.snapshotOperationManager = NewSnapshotBatchManager(ss)
	return ss
}

func (ss *ServiceState) PostEvent(event krunloop.IEvent[*ServiceState]) {
	ss.runloop.PostEvent(event)
}

func (ss *ServiceState) PostActionAndWait(fn func(ss *ServiceState), name string) {
	ch := make(chan struct{})
	eve := NewActionEvent(func(ss *ServiceState) {
		fn(ss)
		close(ch)
	}, name)
	ss.runloop.PostEvent(eve)
	<-ch
}

func (ss *ServiceState) GetRunloopQueueLength() int {
	return ss.runloop.GetQueueLength()
}

// IsResource implements the CriticalResource interface
func (ss *ServiceState) IsResource() {}

func (ss *ServiceState) StopAndWaitForExit(ctx context.Context) {
	if ss.runloop != nil {
		ss.runloop.StopAndWaitForExit()
	}
	if ss.ShadowState != nil {
		ss.ShadowState.StopAndWaitForExit(ctx)
	}
	if ss.SolverGroup != nil {
		ss.SolverGroup.StopAndWaitForExit()
	}
	if ss.ShardPlanWatcher != nil {
		ss.ShardPlanWatcher.StopAndWaitForExit()
	}
	if ss.WorkerEphWatcher != nil {
		ss.WorkerEphWatcher.StopAndWaitForExit()
	}
	if ss.ServiceConfigWatcher != nil {
		ss.ServiceConfigWatcher.StopAndWaitForExit()
	}
}

type FlushScope int // bitmask
const (
	FS_None        FlushScope = 0
	FS_WorkerState FlushScope = 1 << 0
	FS_Pilot       FlushScope = 1 << 1
	FS_Routing     FlushScope = 1 << 2
	// FS_RecreateSnapshot FlushScope = 1 << 3
	FS_Most FlushScope = FS_WorkerState | FS_Pilot | FS_Routing
	FS_All  FlushScope = FS_WorkerState | FS_Pilot | FS_Routing //| FS_RecreateSnapshot
)

// FlushWorkerState: call this to flush all the in-memory state to the store
func (ss *ServiceState) FlushWorkerState(ctx context.Context, workerFullId data.WorkerFullId, workerState *WorkerState, scope FlushScope, reason string) {
	if workerState == nil {
		if scope&FS_WorkerState != 0 {
			ss.storeProvider.StoreWorkerState(workerFullId, nil)
		}
		if scope&FS_Pilot != 0 {
			ss.pilotProvider.StorePilotNode(ctx, workerFullId, nil)
		}
		if scope&FS_Routing != 0 {
			ss.routingProvider.StoreRoutingEntry(ctx, workerFullId, nil)
		}
		// if scope&FS_RecreateSnapshot != 0 {
		// 	ss.reCreateSnapshotBatchManager.TrySchedule(ctx, "FlushWorkerState:"+reason)
		// }
		return
	}
	// workerStateJson
	if scope&FS_WorkerState != 0 {
		workerStateJson := workerState.ToWorkerStateJson(ctx, ss, reason)
		ss.storeProvider.StoreWorkerState(workerFullId, workerStateJson)
	}
	// pilot
	if scope&FS_Pilot != 0 {
		ss.pilotProvider.StorePilotNode(ctx, workerFullId, workerState.ToPilotNode(ctx, ss, reason))
	}
	// routing table
	if scope&FS_Routing != 0 {
		ss.routingProvider.StoreRoutingEntry(ctx, workerFullId, workerState.ToRoutingEntry(ctx, ss, reason))
	}
	// // trigger snapshot recreate
	// if scope&FS_RecreateSnapshot != 0 {
	// 	ss.reCreateSnapshotBatchManager.TrySchedule(ctx, "FlushWorkerState:"+reason) // TODO: most of workerState change should not trigger snapshot recreate
	// }
}

// in case of worker not found, we return nil
func (ss *ServiceState) FindWorkerStateByWorkerFullId(workerFullId data.WorkerFullId) *WorkerState {
	workerState, ok := ss.AllWorkers[workerFullId]
	if !ok {
		return nil
	}
	return workerState
}

func (ss *ServiceState) PrintAllShards(ctx context.Context) {
	for _, shard := range ss.AllShards {
		data, err := json.Marshal(shard)
		if err != nil {
			ke := kerror.Wrap(err, "json.Marshal", "failed to marshal shard", false).With("shardId", shard.ShardId)
			panic(ke)
		}
		klogging.Info(ctx).With("shardId", shard.ShardId).With("data", string(data)).Log("PrintAllShards", "shard")
	}
}
func (ss *ServiceState) PrintAllWorkers(ctx context.Context) {
	for id, workerState := range ss.AllWorkers {
		klogging.Info(ctx).With("workerId", id).With("sessionId", workerState.SessionId).With("data", workerState.ToFullString()).Log("PrintAllWorkers", "worker")
	}
}
func (ss *ServiceState) PrintAllAssignments(ctx context.Context) {
	for _, assignment := range ss.AllAssignments {
		klogging.Info(ctx).With("assignmentId", assignment.AssignmentId).With("assign", assignment.String()).Log("PrintAllAssignments", "assignment")
	}
}

// GetSnapshotCurrentForClone: garenteed return frozen snapshot
func (ss *ServiceState) GetSnapshotCurrentForClone() *costfunc.Snapshot {
	if !ss.SnapshotCurrent.Frozen {
		ss.SnapshotCurrent.Freeze()
	}
	return ss.SnapshotCurrent
}

func (ss *ServiceState) ModifySnapshot(ctx context.Context, fn func(*costfunc.Snapshot), reason string) {
	ss.ModifySnapshotCurrent(ctx, fn, reason)
	ss.ModifySnapshotFuture(ctx, fn, reason)
	ss.broadcastSnapshotBatchManager.TryScheduleInternal(ctx, reason)
}

func (ss *ServiceState) ModifySnapshotCurrent(ctx context.Context, fn func(*costfunc.Snapshot), reason string) {
	if ss.SnapshotCurrent.Frozen {
		ss.SnapshotCurrent = ss.SnapshotCurrent.Clone()
	}
	fn(ss.SnapshotCurrent)
	if klogging.IsVerboseEnabled() {
		klogging.Info(ctx).With("snapshotId", ss.SnapshotCurrent.SnapshotId).With("reason", reason).With("newSnapshot", ss.SnapshotCurrent.ToJsonString()).Log("ModifySnapshotCurrent", "")
	} else {
		klogging.Info(ctx).With("snapshotId", ss.SnapshotCurrent.SnapshotId).With("reason", reason).Log("ModifySnapshotCurrent", "")
	}
}

func (ss *ServiceState) GetSnapshotCurrentForAny() *costfunc.Snapshot {
	return ss.SnapshotCurrent
}

func (ss *ServiceState) ModifySnapshotFuture(ctx context.Context, fn func(*costfunc.Snapshot), reason string) {
	if ss.SnapshotFuture.Frozen {
		ss.SnapshotFuture = ss.SnapshotFuture.Clone()
	}
	fn(ss.SnapshotFuture)
	if klogging.IsVerboseEnabled() {
		klogging.Info(ctx).With("snapshotId", ss.SnapshotFuture.SnapshotId).With("reason", reason).With("newSnapshot", ss.SnapshotFuture.ToJsonString()).Log("ModifySnapshotFuture", "")
	} else {
		klogging.Info(ctx).With("snapshotId", ss.SnapshotFuture.SnapshotId).With("reason", reason).Log("ModifySnapshotFuture", "")
	}
}

func (ss *ServiceState) SetSnapshotCurrent(ctx context.Context, newSnapshot *costfunc.Snapshot, reason string) {
	oldId := "none"
	if ss.SnapshotCurrent != nil {
		oldId = string(ss.SnapshotCurrent.SnapshotId)
	}
	if !newSnapshot.Frozen {
		klogging.Fatal(ctx).With("snapshot", newSnapshot.ToJsonString()).With("reason", reason).Log("SetSnapshotCurrent", "snapshot is not frozen")
	}
	newId := string(newSnapshot.SnapshotId)
	ss.SnapshotCurrent = newSnapshot
	klogging.Debug(ctx).With("oldId", oldId).With("newId", newId).With("reason", reason).WithVerbose("snapshot", newSnapshot.ToJsonString()).Log("SetSnapshotCurrent", "")
}

func (ss *ServiceState) SetSnapshotFuture(ctx context.Context, newSnapshot *costfunc.Snapshot, reason string) {
	oldId := "none"
	if ss.SnapshotFuture != nil {
		oldId = string(ss.SnapshotFuture.SnapshotId)
	}
	if !newSnapshot.Frozen {
		klogging.Fatal(ctx).With("snapshot", newSnapshot.ToJsonString()).With("reason", reason).Log("SetSnapshotFuture", "snapshot is not frozen")
	}
	newId := string(newSnapshot.SnapshotId)
	ss.SnapshotFuture = newSnapshot
	klogging.Debug(ctx).With("oldId", oldId).With("newId", newId).With("reason", reason).WithVerbose("snapshot", newSnapshot.ToJsonString()).Log("SetSnapshotFuture", "")
}

func (ss *ServiceState) GetSnapshotFutureForClone(ctx context.Context) *costfunc.Snapshot {
	if !ss.SnapshotFuture.Frozen {
		ss.SnapshotFuture.Freeze()
	}
	if klogging.IsVerboseEnabled() {
		klogging.Debug(ctx).With("snapshotId", ss.SnapshotFuture.SnapshotId).WithVerbose("snapshot", ss.SnapshotFuture.ToJsonString()).Log("GetSnapshotFutureForClone", "")
	} else {
		klogging.Debug(ctx).With("snapshotId", ss.SnapshotFuture.SnapshotId).Log("GetSnapshotFutureForClone", "")
	}
	return ss.SnapshotFuture
}

func (ss *ServiceState) GetSnapshotFutureForAny(ctx context.Context) *costfunc.Snapshot {
	if klogging.IsVerboseEnabled() {
		klogging.Debug(ctx).With("snapshotId", ss.SnapshotFuture.SnapshotId).WithVerbose("snapshot", ss.SnapshotFuture.ToJsonString()).Log("GetSnapshotFutureForAny", "")
	} else {
		klogging.Debug(ctx).With("snapshotId", ss.SnapshotFuture.SnapshotId).Log("GetSnapshotFutureForAny", "")
	}
	return ss.SnapshotFuture
}
