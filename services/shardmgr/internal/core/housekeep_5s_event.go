package core

import (
	"context"
	"strconv"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
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
		// check assignments tombstone
		for assignId := range workerState.Assignments {
			assignment, ok := ss.AllAssignments[assignId]
			if !ok {
				klogging.Fatal(ctx).With("workerFullId", workerFullId).With("assignId", assignId).Log("checkWorkerTombStone", "assignment not found")
			}
			if assignment.TargetState == cougarjson.CAS_Dropped && assignment.CurrentConfirmedState == cougarjson.CAS_Dropped {
				// delete this assignment
				delete(workerState.Assignments, assignId)
				delete(ss.AllAssignments, assignId)
				delete(ss.AllShards[assignment.ShardId].Replicas[assignment.ReplicaIdx].Assignments, assignId)
				klogging.Info(ctx).With("workerFullId", workerFullId).With("assignId", assignId).Log("checkWorkerTombStone", "delete assignment")
			}
		}
		// check worker tombstone
		if workerState.State == data.WS_Offline_dead {
			// delete all assignments (if any) (rare case, since we should have drained them already. The only case we need to do this when DirtyPurge happened)
			for assignId := range workerState.Assignments {
				assignment, ok := ss.AllAssignments[assignId]
				delete(workerState.Assignments, assignId)
				if !ok {
					continue
				}
				delete(ss.AllAssignments, assignId)
				shardState, ok := ss.AllShards[assignment.ShardId]
				if !ok {
					continue
				}
				replicaState, ok := shardState.Replicas[assignment.ReplicaIdx]
				if !ok {
					continue
				}
				delete(replicaState.Assignments, assignId)
			}
			// delete this worker
			delete(ss.AllWorkers, workerFullId)
			delete(ss.ShutdownHat, workerFullId)
			ss.storeProvider.StoreWorkerState(workerFullId, nil)
			ss.pilotProvider.StorePilotNode(ctx, workerFullId, nil)
			ss.routingProvider.StoreRoutingEntry(ctx, workerFullId, nil)
			klogging.Info(ctx).With("workerFullId", workerFullId).Log("checkWorkerTombStone", "delete workerState")
			continue
		}
	}
}
func (ss *ServiceState) checkWorkerHats(ctx context.Context) {
	// for workerFullId, workerState := range ss.AllWorkers {
	// 	if workerState.IsWaitingForHat() {
	// 	}
	// }
}
func (ws *WorkerState) IsWaitingForHat() bool {
	if ws.State == data.WS_Offline_draining_candidate {
		return true
	}
	if ws.State == data.WS_Online_shutdown_req {
		return true
	}
	return false
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
					ss.hardDeleteReplicaFromSnapshot(ctx, shard.ShardId, replica.ReplicaIdx) // don't forget to delete from (current) snapshot
					dirtyFlag.AddDirtyFlag("hardDeleteReplica:" + string(shard.ShardId) + ":" + strconv.Itoa(int(replica.ReplicaIdx)))
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

func (ss *ServiceState) hardDeleteReplicaFromSnapshot(ctx context.Context, shardId data.ShardId, replicaIdx data.ReplicaIdx) {
	fn := func(snapshot *costfunc.Snapshot) *costfunc.Snapshot {
		newSnapshot := snapshot.Clone()
		shardSnap, ok := newSnapshot.AllShards.Get(shardId)
		if !ok {
			klogging.Fatal(ctx).With("shardId", shardId).Log("checkShardTombStone", "shard not found in snapshot")
		}
		newShardSnap := shardSnap.Clone()
		delete(newShardSnap.Replicas, replicaIdx)
		newSnapshot.AllShards.Set(shardId, newShardSnap)
		return newSnapshot
	}
	ss.SetSnapshotCurrent(ctx, fn(ss.GetSnapshotCurrent()), "hardDeleteReplicaFromSnapshot")
	ss.SnapshotFuture = fn(ss.SnapshotFuture)
}
