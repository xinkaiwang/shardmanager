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
	Name            string
	runloop         *krunloop.RunLoop[*ServiceState]
	PathManager     *config.PathManager
	ServiceInfo     *ServiceInfo
	ServiceConfig   *config.ServiceConfig
	storeProvider   shadow.StoreProvider
	pilotProvider   shadow.PilotProvider
	routingProvider shadow.RoutingProvider
	actionProvider  shadow.ActionProvider
	ShadowState     *shadow.ShadowState
	solverGroup     solver.SnapshotListener

	// Note: all the following fields are not thread-safe, one should never access them outside the runloop.
	AllShards      map[data.ShardId]*ShardState
	AllWorkers     map[data.WorkerFullId]*WorkerState
	AllAssignments map[data.AssignmentId]*AssignmentState

	ProposalQueue   *ProposalQueue
	SnapshotCurrent *costfunc.Snapshot // current means current state
	SnapshotFuture  *costfunc.Snapshot // future = current + in_flight_moves (this is expected future, assume all moves goes well. most solver explore should be based on this.)

	EphDirty         map[data.WorkerFullId]common.Unit
	EphWorkerStaging map[data.WorkerFullId]*cougarjson.WorkerEphJson
	ShutdownHat      map[data.WorkerFullId]common.Unit // those worker with hat means they are in shutdown process

	ShardPlanWatcher *ShardPlanWatcher
	WorkerEphWatcher *WorkerEphWatcher

	syncWorkerBatchManager *BatchManager
}

func NewServiceState(ctx context.Context, name string) *ServiceState { // name is for logging purpose only
	ss := &ServiceState{
		Name:             name,
		AllShards:        make(map[data.ShardId]*ShardState),
		AllWorkers:       make(map[data.WorkerFullId]*WorkerState),
		AllAssignments:   make(map[data.AssignmentId]*AssignmentState),
		EphDirty:         make(map[data.WorkerFullId]common.Unit),
		EphWorkerStaging: make(map[data.WorkerFullId]*cougarjson.WorkerEphJson),
		ShutdownHat:      make(map[data.WorkerFullId]common.Unit),
	}
	ss.PathManager = config.NewPathManager()
	ss.ShadowState = shadow.NewShadowState(ctx, ss.PathManager)
	ss.ProposalQueue = NewProposalQueue(ss, 20)
	ss.storeProvider = ss.ShadowState
	ss.pilotProvider = shadow.NewPilotStore(ss.PathManager)
	ss.routingProvider = shadow.NewDefaultRoutingProvider(ss.PathManager)
	ss.actionProvider = shadow.NewDefaultActionProvider(ss.PathManager)
	ss.runloop = krunloop.NewRunLoop(ctx, ss, "ss")
	ss.Init(ctx)
	// strt runloop
	go ss.runloop.Run(ctx)
	ss.syncWorkerBatchManager = NewBatchManager(ss, 10, "syncWorkerBatch", func(ctx context.Context, ss *ServiceState) {
		ss.syncEphStagingToWorkerState(ctx)
	})
	return ss
}

func (ss *ServiceState) IsStateInMemory() bool {
	// SOFT_STATEFUL = state is in memory = state lost after restarts
	return ss.ServiceInfo.ServiceType == smgjson.ST_SOFT_STATEFUL
}

func (ss *ServiceState) PostEvent(event krunloop.IEvent[*ServiceState]) {
	ss.runloop.PostEvent(event)
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
}
