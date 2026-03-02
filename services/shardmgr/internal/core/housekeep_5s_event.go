package core

import (
	"os"
	"log/slog"
	"context"
	"strconv"

	"github.com/xinkaiwang/shardmanager/libs/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kmetrics"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

// Housekeep5sEvent implements krunloop.IEvent[*ServiceState] interface
type Housekeep5sEvent struct {
	createTimeMs int64 // time when the event was created
}

func (te *Housekeep5sEvent) GetCreateTimeMs() int64 {
	return te.createTimeMs
}
func (te *Housekeep5sEvent) GetName() string {
	return "Housekeep5sEvent"
}

func (te *Housekeep5sEvent) Process(ctx context.Context, ss *ServiceState) {
	ke := kcommon.TryCatchRun(ctx, func() {
		kmetrics.InstrumentSummaryRunVoid(ctx, "checkWorkerTombStone", func() {
			ss.checkWorkerTombStone(ctx)
		}, "none")
		kmetrics.InstrumentSummaryRunVoid(ctx, "checkShardTombStone", func() {
			ss.checkShardTombStone(ctx)
		}, "none")
		kmetrics.InstrumentSummaryRunVoid(ctx, "collectXxxxStats", func() {
			ss.collectWorkerStats(ctx)
			ss.collectShardStats(ctx)
			ss.collectCurrentScore(ctx)
		}, "none")
	})
	if ke != nil {
		slog.ErrorContext(ctx, "checkWorkerTombStone failed",
			slog.String("event", "Housekeep5sEvent"),
			slog.Any("error", ke))
	}
	kcommon.ScheduleRun(5*1000, func() { // 5s
		ss.PostEvent(NewHousekeep5sEvent())
	})
}

func NewHousekeep5sEvent() *Housekeep5sEvent {
	return &Housekeep5sEvent{
		createTimeMs: kcommon.GetWallTimeMs(),
	}
}

func (ss *ServiceState) checkWorkerTombStone(ctx context.Context) {
	var passiveMoves []costfunc.PassiveMove
	for workerFullId, workerState := range ss.AllWorkers {

		// check assignments tombstone
		for _, assignId := range workerState.Assignments {
			assignment, ok := ss.AllAssignments[assignId]
			if !ok {
				slog.ErrorContext(ctx, "assignment not found",
					slog.String("event", "checkWorkerTombStone"),
					slog.Any("workerFullId", workerFullId),
					slog.Any("assignId", assignId))
				os.Exit(1)
			}
			if assignment.TargetState == cougarjson.CAS_Dropped && assignment.CurrentConfirmedState == cougarjson.CAS_Dropped {
				passiveMove := NewPasMoveRemoveAssignment(assignId, assignment.ShardId, assignment.ReplicaIdx, workerFullId)
				passiveMove.ApplyToSs(ctx, ss)
				// passiveMoves = append(passiveMoves, passiveMove) // assignments in snapshot should have been removed already, don't need to do it again
				slog.InfoContext(ctx, "delete assignment",
					slog.String("event", "checkWorkerTombStone"),
					slog.Any("workerFullId", workerFullId),
					slog.Any("assignId", assignId))
			}
		}
		// check worker tombstone
		if workerState.State == data.WS_Offline_dead {
			// [defensive coding] delete all assignments (if any) (rare case, since we should have drained them already. The only case we need to do this when DirtyPurge happened)
			for _, assignId := range workerState.Assignments {
				// delete(workerState.Assignments, assignId)
				assignment, ok := ss.AllAssignments[assignId]
				if !ok {
					continue
				}
				passiveMove := NewPasMoveRemoveAssignment(assignId, assignment.ShardId, assignment.ReplicaIdx, assignment.WorkerFullId)
				passiveMove.ApplyToSs(ctx, ss)
				passiveMoves = append(passiveMoves, passiveMove)
			}
			// delete this worker
			delete(ss.AllWorkers, workerFullId)
			delete(ss.ShutdownHat, workerFullId)
			ss.storeProvider.StoreWorkerState(workerFullId, nil)
			ss.pilotProvider.StorePilotNode(ctx, workerFullId, nil)
			ss.routingProvider.StoreRoutingEntry(ctx, workerFullId, nil)
			passiveMove := costfunc.NewPasMoveWorkerSnapAddRemove(workerFullId, nil, "hardDeleteWorker")
			passiveMoves = append(passiveMoves, passiveMove)
			slog.InfoContext(ctx, "delete workerState",
				slog.String("event", "checkWorkerTombStone"),
				slog.Any("workerFullId", workerFullId))
			continue
		}
	}
	if len(passiveMoves) > 0 {
		for _, passiveMove := range passiveMoves {
			ss.ModifySnapshot(ctx, passiveMove.Apply, passiveMove.Signature())
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
					if replica.cleanupStartTimeMs == 0 {
						replica.cleanupStartTimeMs = kcommon.GetWallTimeMs()
					} else if kcommon.GetWallTimeMs()-replica.cleanupStartTimeMs > 30*1000 { // hard-delete lameduck replica after 30s
						// delete this replica
						// klogging.Info(ctx).With("shardId", shard.ShardId).With("replicaIdx", replica.ReplicaIdx).Log("checkShardTombStone", "delete replica")
						delete(shard.Replicas, replica.ReplicaIdx)
						passiveMove := costfunc.NewPasMoveReplicaSnapHardDelete(shard.ShardId, replica.ReplicaIdx)
						ss.ModifySnapshot(ctx, passiveMove.Apply, "hardDeleteReplica")
						dirtyFlag.AddDirtyFlag("hardDeleteReplica:" + string(shard.ShardId) + ":" + strconv.Itoa(int(replica.ReplicaIdx)))
					}
				}
			}
		}
		if shard.LameDuck && len(shard.Replicas) == 0 {
			// shard is tombstone, delete it
			delete(ss.AllShards, shard.ShardId)
			ss.storeProvider.StoreShardState(shard.ShardId, nil)
			slog.InfoContext(ctx, "delete shardState",
				slog.String("event", "hardDeleteShardState"),
				slog.Any("shardId", shard.ShardId))
			// delete from snapshot
			passiveMove := costfunc.NewPasMoveShardStateAddRemove(shard.ShardId, nil, "hardDeleteShard")
			ss.ModifySnapshot(ctx, passiveMove.Apply, "hardDeleteShard")
		} else if dirtyFlag.IsDirty() {
			// shard is dirty, update it
			shard.LastUpdateTimeMs = kcommon.GetWallTimeMs()
			shard.LastUpdateReason = dirtyFlag.String()
			ss.storeProvider.StoreShardState(shard.ShardId, shard.ToJson())

			slog.InfoContext(ctx, "update shardState",
				slog.String("event", "checkShardTombStone"),
				slog.Any("shardId", shard.ShardId),
				slog.Any("updateReason", shard.LastUpdateReason))
		}
	}
}
