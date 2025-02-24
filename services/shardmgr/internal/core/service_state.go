package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

type ServiceState struct {
	runloop     *RunLoop
	ServiceInfo *ServiceInfo

	// Note: all the following fields are not thread-safe, one should never access them outside the runloop.
	AllShards  map[data.ShardId]*ShardState
	AllWorkers map[data.WorkerFullId]*WorkerState

	PathManager      *PathManager
	ShardPlanWatcher *ShardPlanWatcher
	WorkerEphWatcher *WorkerEphWatcher
}

func NewServiceState(ctx context.Context) *ServiceState {
	ss := &ServiceState{
		AllShards:  map[data.ShardId]*ShardState{},
		AllWorkers: map[data.WorkerFullId]*WorkerState{},
	}
	ss.runloop = NewRunLoop(ctx, ss)
	ss.PathManager = NewPathManager()
	ss.Init(ctx)
	// strt runloop
	go ss.runloop.Run(ctx)
	return ss
}

func (ss *ServiceState) IsStateInMemory() bool {
	// SOFT_STATEFUL = state is in memory = state lost after restarts
	return ss.ServiceInfo.ServiceType == smgjson.ST_SOFT_STATEFUL
}
