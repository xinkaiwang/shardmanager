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
}

func NewServiceState(ctx context.Context) *ServiceState {
	ss := &ServiceState{}
	ss.Init(ctx)
	return ss
}

func (ss *ServiceState) IsStateInMemory() bool {
	// SOFT_STATEFUL = state is in memory = state lost after restarts
	return ss.ServiceInfo.ServiceType == smgjson.ST_SOFT_STATEFUL
}
