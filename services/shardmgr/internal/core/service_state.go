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
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

// ServiceState implements the CriticalResource interface
type ServiceState struct {
	runloop       *krunloop.RunLoop[*ServiceState]
	PathManager   *config.PathManager
	ServiceInfo   *ServiceInfo
	ServiceConfig *config.ServiceConfig
	StoreProvider shadow.StoreProvider
	ShadowState   *shadow.ShadowState

	// Note: all the following fields are not thread-safe, one should never access them outside the runloop.
	AllShards      map[data.ShardId]*ShardState
	AllWorkers     map[data.WorkerFullId]*WorkerState
	AllAssignments map[data.AssignmentId]*AssignmentState

	SnapshotCurrent *costfunc.Snapshot // current means current state
	SnapshotFuture  *costfunc.Snapshot // future = current + in_flight_moves (this is expected future, assume all moves goes well. most solver explore should be based on this.)

	EphDirty         map[data.WorkerFullId]common.Unit
	EphWorkerStaging map[data.WorkerFullId]*cougarjson.WorkerEphJson

	ShardPlanWatcher *ShardPlanWatcher
	WorkerEphWatcher *WorkerEphWatcher
}

func NewServiceState(ctx context.Context) *ServiceState {
	ss := &ServiceState{
		AllShards:        make(map[data.ShardId]*ShardState),
		AllWorkers:       make(map[data.WorkerFullId]*WorkerState),
		AllAssignments:   make(map[data.AssignmentId]*AssignmentState),
		EphDirty:         make(map[data.WorkerFullId]common.Unit),
		EphWorkerStaging: make(map[data.WorkerFullId]*cougarjson.WorkerEphJson),
	}
	ss.PathManager = config.NewPathManager()
	ss.ShadowState = shadow.NewShadowState(ctx, ss.PathManager)
	ss.StoreProvider = ss.ShadowState
	ss.runloop = krunloop.NewRunLoop(ctx, ss, "ss")
	ss.Init(ctx)
	// strt runloop
	go ss.runloop.Run(ctx)
	return ss
}

func (ss *ServiceState) IsStateInMemory() bool {
	// SOFT_STATEFUL = state is in memory = state lost after restarts
	return ss.ServiceInfo.ServiceType == smgjson.ST_SOFT_STATEFUL
}

func (ss *ServiceState) EnqueueEvent(event krunloop.IEvent[*ServiceState]) {
	ss.runloop.EnqueueEvent(event)
}

// IsResource implements the CriticalResource interface
func (ss *ServiceState) IsResource() {}
