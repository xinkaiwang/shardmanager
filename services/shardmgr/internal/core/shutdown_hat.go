package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

// return true means you get a hat
func (ss *ServiceState) hatTryGet(ctx context.Context, workerFullId data.WorkerFullId) bool {
	if len(ss.ShutdownHat) >= int(ss.ServiceConfig.SystemLimit.MaxHatCountLimit) {
		return false
	}
	ss.ShutdownHat[workerFullId] = common.Unit{}
	return true
}

func (ss *ServiceState) hatReturn(ctx context.Context, workerFullId data.WorkerFullId) {
	delete(ss.ShutdownHat, workerFullId)
}
