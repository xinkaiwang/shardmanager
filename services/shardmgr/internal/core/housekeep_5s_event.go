package core

import (
	"context"
	"strconv"

	"github.com/xinkaiwang/shardmanager/libs/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
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
		ss.collectWorkerStats(ctx)
		ss.collectShardStats(ctx)
		ss.collectCurrentScore(ctx)
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
	var passiveMoves []costfunc.PassiveMove
	for workerFullId, workerState := range ss.AllWorkers {

		// check assignments tombstone
		for assignId := range workerState.Assignments {
			assignment, ok := ss.AllAssignments[assignId]
			if !ok {
				klogging.Fatal(ctx).With("workerFullId", workerFullId).With("assignId", assignId).Log("checkWorkerTombStone", "assignment not found")
			}
			if assignment.TargetState == cougarjson.CAS_Dropped && assignment.CurrentConfirmedState == cougarjson.CAS_Dropped {
				// // delete this assignment
				// delete(workerState.Assignments, assignId)
				// delete(ss.AllAssignments, assignId)
				// delete(ss.AllShards[assignment.ShardId].Replicas[assignment.ReplicaIdx].Assignments, assignId)
				// passiveMove := costfunc.NewPasMoveWorkerSnapUpdate(workerFullId, func(ws *costfunc.WorkerSnap) {
				// 	delete(ws.Assignments, assignment.ShardId)
				// }, "hard delete assignment")
				passiveMove := NewPasMoveRemoveAssignment(assignId, assignment.ShardId, assignment.ReplicaIdx, workerFullId)
				passiveMove.ApplyToSs(ss)
				passiveMoves = append(passiveMoves, passiveMove)
				klogging.Info(ctx).With("workerFullId", workerFullId).With("assignId", assignId).Log("checkWorkerTombStone", "delete assignment")
			}
		}
		// check worker tombstone
		if workerState.State == data.WS_Offline_dead {
			// [defensive coding] delete all assignments (if any) (rare case, since we should have drained them already. The only case we need to do this when DirtyPurge happened)
			for assignId := range workerState.Assignments {
				// delete(workerState.Assignments, assignId)
				assignment, ok := ss.AllAssignments[assignId]
				if !ok {
					continue
				}
				passiveMove := NewPasMoveRemoveAssignment(assignId, assignment.ShardId, assignment.ReplicaIdx, assignment.WorkerFullId)
				passiveMove.ApplyToSs(ss)
				passiveMoves = append(passiveMoves, passiveMove)
				// delete(ss.AllAssignments, assignId)
				// shardState, ok := ss.AllShards[assignment.ShardId]
				// if !ok {
				// 	continue
				// }
				// replicaState, ok := shardState.Replicas[assignment.ReplicaIdx]
				// if !ok {
				// 	continue
				// }
				// delete(replicaState.Assignments, assignId)
			}
			// delete this worker
			delete(ss.AllWorkers, workerFullId)
			delete(ss.ShutdownHat, workerFullId)
			ss.storeProvider.StoreWorkerState(workerFullId, nil)
			ss.pilotProvider.StorePilotNode(ctx, workerFullId, nil)
			ss.routingProvider.StoreRoutingEntry(ctx, workerFullId, nil)
			passiveMove := costfunc.NewPasMoveWorkerSnapAddRemove(workerFullId, nil, "hardDeleteWorker")
			passiveMoves = append(passiveMoves, passiveMove)
			klogging.Info(ctx).With("workerFullId", workerFullId).Log("checkWorkerTombStone", "delete workerState")
			continue
		}
	}
	if len(passiveMoves) > 0 {
		for _, passiveMove := range passiveMoves {
			ss.ModifySnapshot(ctx, passiveMove.Apply, "hardDeleteWorker")
		}
	}
}

func (ss *ServiceState) collectWorkerStats(ctx context.Context) {
	var workerCountTotal int64
	var workerCountShutdownReq int64
	var workerCountDraining int64
	var workerCountOffline int64
	var workerCountOnline int64

	for _, workerState := range ss.AllWorkers {
		// metrics
		workerCountTotal++
		if workerState.IsOnline() {
			workerCountOnline++
		}
		if workerState.IsOffline() {
			workerCountOffline++
		}
		if workerState.ShutdownRequesting {
			workerCountShutdownReq++
		}
		if workerState.HasShutdownHat() {
			workerCountDraining++
		}
	}

	ss.MetricsValueWorkerCount_total.Store(workerCountTotal)
	ss.MetricsValueWorkerCount_online.Store(workerCountOnline)
	ss.MetricsValueWorkerCount_offline.Store(workerCountOffline)
	ss.MetricsValueWorkerCount_draining.Store(workerCountDraining)
	ss.MetricsValueWorkerCount_shutdownReq.Store(workerCountShutdownReq)
}

func (ss *ServiceState) collectShardStats(ctx context.Context) {
	shardCountTotal := int64(len(ss.AllShards))
	var replicaCountTotal int64
	assignmentCount := int64(len(ss.AllAssignments))

	for _, shardState := range ss.AllShards {
		replicaCountTotal += int64(len(shardState.Replicas))
	}
	ss.MetricsValueShardCount.Store(shardCountTotal)
	ss.MetricsValueReplicaCount.Store(replicaCountTotal)
	ss.MetricsValueAssignmentCount.Store(assignmentCount)
}

func (ss *ServiceState) collectCurrentScore(ctx context.Context) {
	// collect current score
	if ss.SnapshotCurrent != nil {
		currentCost := ss.SnapshotCurrent.GetCost()
		ss.MetricsValueCurrentSoftCost.Store(int64(currentCost.SoftScore))
		ss.MetricsValueCurrentHardCost.Store(int64(currentCost.HardScore))
	} else {
		ss.MetricsValueCurrentSoftCost.Store(0)
		ss.MetricsValueCurrentHardCost.Store(0)
	}
	if ss.SnapshotFuture != nil {
		futureCost := ss.SnapshotFuture.GetCost()
		ss.MetricsValueFutureSoftCost.Store(int64(futureCost.SoftScore))
		ss.MetricsValueFutureHardCost.Store(int64(futureCost.HardScore))
	} else {
		ss.MetricsValueFutureSoftCost.Store(0)
		ss.MetricsValueFutureHardCost.Store(0)
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
					passiveMove := costfunc.NewPasMoveReplicaSnapAddRemove(shard.ShardId, replica.ReplicaIdx, false)
					ss.ModifySnapshot(ctx, passiveMove.Apply, "hardDeleteReplica")
					dirtyFlag.AddDirtyFlag("hardDeleteReplica:" + string(shard.ShardId) + ":" + strconv.Itoa(int(replica.ReplicaIdx)))
				}
			}
		}
		if shard.LameDuck && len(shard.Replicas) == 0 {
			// shard is tombstone, delete it
			delete(ss.AllShards, shard.ShardId)
			ss.storeProvider.StoreShardState(shard.ShardId, nil)
			klogging.Info(ctx).With("shardId", shard.ShardId).Log("hardDeleteShardState", "delete shardState")
			// delete from snapshot
			passiveMove := costfunc.NewPasMoveShardStateAddRemove(shard.ShardId, nil, "hardDeleteShard")
			ss.ModifySnapshot(ctx, passiveMove.Apply, "hardDeleteShard")
		} else if dirtyFlag.IsDirty() {
			// shard is dirty, update it
			shard.LastUpdateTimeMs = kcommon.GetWallTimeMs()
			shard.LastUpdateReason = dirtyFlag.String()
			ss.storeProvider.StoreShardState(shard.ShardId, shard.ToJson())

			klogging.Info(ctx).With("shardId", shard.ShardId).With("updateReason", shard.LastUpdateReason).Log("checkShardTombStone", "update shardState")
		}
	}
}
