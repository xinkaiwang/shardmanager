package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/krunloop"
	"github.com/xinkaiwang/shardmanager/services/cougar/cougarjson"
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

	syncWorkerBatchManager       *BatchManager // enqueue when any worker eph changed, dequeue= trigger ss.syncEphStagingToWorkerState()
	reCreateSnapshotBatchManager *BatchManager // enqueue when any workerState/shardState add/remove/etc. dequeue=trigger snapshot recreate
	syncShardsBatchManager       *BatchManager // enqueue when 1) shard plan new/changed, 2) shard config changed, etc. dequeue=trigger ss.syncShardPlan()
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
	ss.syncWorkerBatchManager = NewBatchManager(ss, 10, "syncWorkerBatch", func(ctx context.Context, ss *ServiceState) {
		ss.digestStagingWorkerEph(ctx)
	})
	ss.reCreateSnapshotBatchManager = NewBatchManager(ss, 10, "reCreateSnapshotBatch", func(ctx context.Context, ss *ServiceState) {
		ss.ReCreateSnapshot(ctx)
	})
	ss.syncShardsBatchManager = NewBatchManager(ss, 10, "syncShardsBatch", func(ctx context.Context, ss *ServiceState) {
		ss.digestStagingShardPlan(ctx)
	})
	return ss
}

func (ss *ServiceState) PostEvent(event krunloop.IEvent[*ServiceState]) {
	ss.runloop.PostEvent(event)
}

func (ss *ServiceState) PostActionAndWait(fn func(ss *ServiceState)) {
	ch := make(chan struct{})
	eve := NewActionEvent(func(ss *ServiceState) {
		fn(ss)
		close(ch)
	})
	ss.runloop.PostEvent(eve)
	<-ch
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
}

// FlushWorkerState: call this to flush all the in-memory state to the store
func (ss *ServiceState) FlushWorkerState(ctx context.Context, workerFullId data.WorkerFullId, workerState *WorkerState, reason string) {
	if workerState == nil {
		ss.storeProvider.StoreWorkerState(workerFullId, nil)
		ss.pilotProvider.StorePilotNode(ctx, workerFullId, nil)
		ss.routingProvider.StoreRoutingEntry(ctx, workerFullId, nil)
		return
	}
	// workerStateJson
	workerStateJson := workerState.ToWorkerStateJson(ctx, ss, reason)
	ss.storeProvider.StoreWorkerState(workerFullId, workerStateJson)
	// pilot
	ss.pilotProvider.StorePilotNode(ctx, workerFullId, workerState.ToPilotNode(ctx, ss, reason))
	// routing table
	ss.routingProvider.StoreRoutingEntry(ctx, workerFullId, workerState.ToRoutingEntry(ctx, ss, reason))
	// trigger snapshot recreate
	ss.reCreateSnapshotBatchManager.TrySchedule(ctx)
}

// in case of worker not found, we return nil
func (ss *ServiceState) FindWorkerStateByWorkerFullId(workerFullId data.WorkerFullId) *WorkerState {
	workerState, ok := ss.AllWorkers[workerFullId]
	if !ok {
		return nil
	}
	return workerState
}
