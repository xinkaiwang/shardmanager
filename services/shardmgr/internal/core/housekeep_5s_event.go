package core

import (
	"context"
	"strconv"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

// Housekeep5sEvent implements krunloop.IEvent[*ServiceState] interface
type Housekeep5sEvent struct {
}

func (te *Housekeep5sEvent) GetName() string {
	return "TimerEvent"
}

func (te *Housekeep5sEvent) Process(ctx context.Context, ss *ServiceState) {
	ke := kcommon.TryCatchRun(ctx, func() {
		ss.checkWorkerTombStone(ctx)
		ss.checkShardTombStone(ctx)
	})
	if ke != nil {
		klogging.Error(ctx).With("error", ke).Log("Housekeep5sEvent", "checkWorkerTombStone failed")
	}
	kcommon.ScheduleRun(5*1000, func() { // 5s
		ss.PostEvent(NewHousekeep5sEvent())
	})
}

func NewHousekeep5sEvent() *Housekeep5sEvent {
	return &Housekeep5sEvent{}
}

func (ss *ServiceState) checkWorkerTombStone(ctx context.Context) {
	for workerFullId, workerState := range ss.AllWorkers {
		if workerState.State == data.WS_Offline_dead {
			delete(ss.AllWorkers, workerFullId)
			ss.storeProvider.StoreWorkerState(workerFullId, nil)
			ss.pilotProvider.StorePilotNode(ctx, workerFullId, nil)
			ss.routingProvider.StoreRoutingEntry(ctx, workerFullId, nil)
			klogging.Info(ctx).With("workerFullId", workerFullId).Log("checkWorkerTombStone", "delete workerState")
		}
	}
}

func (ss *ServiceState) checkShardTombStone(ctx context.Context) {
	// check shard tombstone
	for _, shard := range ss.AllShards {
		dirtyFlag := NewDirtyFlag()
		// cleanup lame duck replicas
		for _, replica := range shard.Replicas {
			if replica.LameDuck {
				// hard delete this if no assignment left
				if len(replica.Assignments) == 0 {
					// delete this replica
					// klogging.Info(ctx).With("shardId", shard.ShardId).With("replicaIdx", replica.ReplicaIdx).Log("checkShardTombStone", "delete replica")
					delete(shard.Replicas, replica.ReplicaIdx)
					dirtyFlag.AddDirtyFlag("hardDeleteReplica" + strconv.Itoa(int(replica.ReplicaIdx)))
				}
			}
		}
		if shard.LameDuck && len(shard.Replicas) == 0 {
			// shard is tombstone, delete it
			delete(ss.AllShards, shard.ShardId)
			ss.storeProvider.StoreShardState(shard.ShardId, nil)
			klogging.Info(ctx).With("shardId", shard.ShardId).Log("checkShardTombStone", "delete shardState")
		} else if dirtyFlag.IsDirty() {
			// shard is dirty, update it
			shard.LastUpdateTimeMs = kcommon.GetWallTimeMs()
			shard.LastUpdateReason = dirtyFlag.String()
			ss.storeProvider.StoreShardState(shard.ShardId, shard.ToJson())
			klogging.Info(ctx).With("shardId", shard.ShardId).With("updateReason", shard.LastUpdateReason).Log("checkShardTombStone", "update shardState")
		}
	}
}
