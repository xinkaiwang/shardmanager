package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

type ServiceState struct {
	runloop *RunLoop

	AllShards  map[data.ShardId]*ShardState
	AllWorkers map[data.WorkerFullId]*WorkerState
}

func NewServiceState(ctx context.Context) *ServiceState {
	ss := &ServiceState{}
	ss.runloop = NewRunLoop(ctx, ss)
	return ss
}
