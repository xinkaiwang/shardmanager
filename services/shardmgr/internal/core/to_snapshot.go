package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

func (ss *ServiceState) ToSnapshot(ctx context.Context) *costfunc.Snapshot {
	// build snapshot based on current ss
	snapshot := costfunc.NewSnapshot(ctx, ss.ServiceConfig.CostFuncCfg)
	// shards
	for shardId, shardState := range ss.AllShards {
		shardSnap := costfunc.NewShardSnap(shardId, 0)
		shardSnap.TargetReplicaCount = shardState.TargetReplicaCount
		for replicaIdx, replicaState := range shardState.Replicas {
			replicaSnap := costfunc.NewReplicaSnap(shardId, replicaIdx)
			replicaSnap.LameDuck = replicaState.LameDuck
			for assignmentId := range replicaState.Assignments {
				replicaSnap.Assignments[assignmentId] = common.Unit{}
			}
			shardSnap.Replicas[replicaIdx] = replicaSnap
		}
		snapshot.AllShards.Set(shardId, shardSnap)
	}
	// workers
	for workerId, workerState := range ss.AllWorkers {
		workerSnap := costfunc.NewWorkerSnap(workerId)
		workerSnap.Draining = workerState.IsDaining()
		workerSnap.Offline = workerState.IsOffline()
		for _, assignmentId := range workerState.Assignments {
			assignment, ok := ss.AllAssignments[assignmentId]
			if !ok {
				// assignment not found
				continue
			}
			workerSnap.Assignments[assignment.ShardId] = assignmentId
		}
		snapshot.AllWorkers.Set(workerId, workerSnap)
	}
	// assignments
	for assignmentId, assignmentState := range ss.AllAssignments {
		assignmentSnap := costfunc.NewAssignmentSnap(assignmentState.ShardId, assignmentState.ReplicaIdx, assignmentId, assignmentState.WorkerFullId)
		snapshot.AllAssignments.Set(assignmentId, assignmentSnap)
	}
	// compact
	snapshot.AllShards = snapshot.AllShards.Compact()
	snapshot.AllWorkers = snapshot.AllWorkers.Compact()
	snapshot.AllAssignments = snapshot.AllAssignments.Compact()

	return snapshot
}

func (ws *WorkerState) IsDaining() bool {
	if ws.ShutdownRequesting {
		return true
	}
	if ws.HasShutdownHat() {
		return true
	}
	if ws.State == data.WS_Online_shutdown_permit || ws.State == data.WS_Offline_draining_complete || ws.State == data.WS_Offline_dead || ws.State == data.WS_Deleted {
		return true
	}
	return false
}

func (ws *WorkerState) IsOffline() bool {
	if ws.State == data.WS_Offline_graceful_period || ws.State == data.WS_Offline_draining_candidate || ws.State == data.WS_Offline_draining_complete || ws.State == data.WS_Offline_dead || ws.State == data.WS_Deleted {
		return true
	}
	return false
}
