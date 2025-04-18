package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/etcdprov"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/solver"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

func (ss *ServiceState) Init(ctx context.Context) {
	klogging.Info(ctx).Log("ServiceStateInit", "start")
	// step 1: load ServiceInfo
	// ss.ServiceInfo = ss.LoadServiceInfo(ctx)     // from /smg/config/service_info.json
	var currentServiceConfigRevision etcdprov.EtcdRevision
	ss.ServiceConfig, currentServiceConfigRevision = ss.LoadServiceConfig(ctx) // from /smg/config/service_config.json
	solver.GetCurrentSolverConfigProvider().OnSolverConfigChange(&ss.ServiceConfig.SolverConfig)
	ss.DynamicThreshold = NewDynamicThreshold(func() config.DynamicThresholdConfig {
		return ss.ServiceConfig.DynamicThresholdConfig
	})

	// setp 2: load all worker state
	ss.LoadAllWorkerState(ctx) // (from /smg/worker_state/ to ss.AllWorkers and ss.AllAssignments)
	// step 3: load all shard state (note: we need to load workerStates first, because workerState will populate assignments)
	ss.LoadAllShardState(ctx) // (from /smg/shard_state/ to ss.AllShards)
	ss.ShadowState.InitDone()
	ss.PrintAllShards(ctx)
	ss.PrintAllWorkers(ctx)
	ss.PrintAllAssignments(ctx)
	klogging.Info(ctx).Log("ServiceStateInit", "load all shard and worker state done")

	// step 4: load current shard plan
	currentShardPlan, currentShardPlanRevision := ss.LoadCurrentShardPlan(ctx)
	ss.stagingShardPlan = currentShardPlan // staging area, will be used in ss.syncShardPlan
	ss.digestStagingShardPlan(ctx)         // based on crrentShardPlan, update ss.AllShards, and write to etcd
	// step 5: load current worker eph
	currentWorkerEph, currentWorkerEphRevision := ss.LoadCurrentWorkerEph(ctx)
	ss.batchAddToStagingWorkerEph(ctx, currentWorkerEph) // staging area, will be used in ss.syncEphStagingToWorkerState
	// step 6: sync workerEph to workerState
	ss.firstDigestStagingWorkerEph(ctx)

	// step 7: current snapshot and future snapshot
	ss.ReCreateSnapshot(ctx)

	// step 8: start listening to shard plan changes/worker eph changes/service config changes
	ss.ShardPlanWatcher = NewShardPlanWatcher(ctx, ss, currentShardPlanRevision+1) // +1 to skip the current revision
	ss.WorkerEphWatcher = NewWorkerEphWatcher(ctx, ss, currentWorkerEphRevision+1)
	ss.ServiceConfigWatcher = NewServiceConfigWatcher(ctx, ss, currentServiceConfigRevision+1)
	ss.ServiceConfigWatcher.SolverConfigListener = append(ss.ServiceConfigWatcher.SolverConfigListener, func(sc *config.SolverConfig) {
		solver.GetCurrentSolverConfigProvider().OnSolverConfigChange(sc)
	})
	ss.ServiceConfigWatcher.ShardConfigListener = append(ss.ServiceConfigWatcher.ShardConfigListener, func(sc *config.ShardConfig) {
		ss.syncShardsBatchManager.TrySchedule(ctx, "ShardConfigWatcher")
	})

	// step 9: start housekeeping threads
	ss.PostEvent(NewHousekeep1sEvent())
	ss.PostEvent(NewHousekeep5sEvent())
	ss.PostEvent(NewAcceptEvent())

	// step 10: start
	// Note: The runloop is now initialized in AssembleSsXxxx
	klogging.Info(ctx).Log("ServiceStateInit", "done")
}

func (ss *ServiceState) LoadAllShardState(ctx context.Context) {
	pathPrefix := ss.PathManager.GetShardStatePathPrefix()
	// load all from etcd
	list, _ := etcdprov.GetCurrentEtcdProvider(ctx).LoadAllByPrefix(ctx, pathPrefix)
	for _, item := range list {
		shardStateJson := smgjson.ShardStateJsonFromJson(item.Value)
		ss.ShadowState.InitShardState(shardStateJson.ShardName, shardStateJson)
		shardState := NewShardStateByJson(ctx, ss, shardStateJson)

		// 设置软删除状态，使用辅助函数转换
		shardState.LameDuck = smgjson.Int82Bool(shardStateJson.LameDuck)

		ss.AllShards[data.ShardId(shardState.ShardId)] = shardState
	}
	// populate assignments in replicas
	for assignId, assignment := range ss.AllAssignments {
		shardState, ok := ss.AllShards[assignment.ShardId]
		if !ok {
			klogging.Fatal(ctx).With("assignmentId", assignId).With("shardId", assignment.ShardId).Log("LoadAllShardState", "shard not found")
			continue
		}
		replicaState, ok := shardState.Replicas[assignment.ReplicaIdx]
		if !ok {
			klogging.Fatal(ctx).With("assignmentId", assignId).With("shardId", assignment.ShardId).With("replicaIdx", assignment.ReplicaIdx).Log("LoadAllShardState", "replica not found")
			continue
		}
		replicaState.Assignments[assignId] = common.Unit{}
	}
}

func (ss *ServiceState) LoadAllWorkerState(ctx context.Context) {
	pathPrefix := ss.PathManager.GetWorkerStatePathPrefix()
	// load all from etcd
	list, _ := etcdprov.GetCurrentEtcdProvider(ctx).LoadAllByPrefix(ctx, pathPrefix)
	for _, item := range list {
		workerStateJson := smgjson.WorkerStateJsonFromJson(item.Value)
		WorkerState := NewWorkerStateFromJson(ss, workerStateJson)
		workerFullId := data.NewWorkerFullId(data.WorkerId(workerStateJson.WorkerId), data.SessionId(workerStateJson.SessionId), data.StatefulType(workerStateJson.StatefulType))
		if WorkerState.HasShutdownHat() {
			ss.ShutdownHat[workerFullId] = common.Unit{}
		}
		ss.ShadowState.InitWorkerState(workerFullId, workerStateJson)
		ss.AllWorkers[workerFullId] = WorkerState
		// assignments
		for assignmentId, assignement := range workerStateJson.Assignments {
			ss.AllAssignments[assignmentId] = NewAssignmentState(assignmentId, assignement.ShardId, assignement.ReplicaIdx, workerFullId)
		}
	}
}

func (ss *ServiceState) LoadCurrentShardPlan(ctx context.Context) ([]*smgjson.ShardLineJson, etcdprov.EtcdRevision) {
	path := ss.PathManager.GetShardPlanPath()
	item := etcdprov.GetCurrentEtcdProvider(ctx).Get(ctx, path)
	list := smgjson.ParseShardPlan(item.Value)
	return list, item.ModRevision
}

func (ss *ServiceState) ReCreateSnapshot(ctx context.Context) {
	ss.SnapshotCurrent = ss.CreateSnapshotFromCurrentState(ctx)
	ss.SnapshotFuture = ss.SnapshotCurrent.Clone()
	// apply all panding moves
	for _, minion := range ss.AllMoves {
		minion.moveState.ApplyRemainingActions(ss.SnapshotFuture, costfunc.AM_Relaxed)
	}
	ss.SnapshotFuture.Freeze()
	if ss.SolverGroup != nil {
		ss.SolverGroup.OnSnapshot(ctx, ss.SnapshotFuture)
		klogging.Info(ctx).With("snapshot", ss.SnapshotFuture.ToShortString()).With("time", kcommon.GetWallTimeMs()).Log("ReCreateSnapshot", "SolverGroup.OnSnapshot")
	} else {
		klogging.Info(ctx).With("snapshot", ss.SnapshotFuture.ToShortString()).With("time", kcommon.GetWallTimeMs()).Log("ReCreateSnapshot", "SolverGroup is nil, skip OnSnapshot")
	}
}

func (ss *ServiceState) CreateSnapshotFromCurrentState(ctx context.Context) *costfunc.Snapshot {
	snapshot := costfunc.NewSnapshot(ctx, ss.ServiceConfig.CostFuncCfg)
	// all shards
	for _, shard := range ss.AllShards {
		shardSnap := costfunc.NewShardSnap(shard.ShardId)
		for replicaIdx, replica := range shard.Replicas {
			replicaSnap := costfunc.NewReplicaSnap(shard.ShardId, replicaIdx)
			replicaSnap.LameDuck = replica.LameDuck
			shardSnap.Replicas[replicaIdx] = replicaSnap
			for assignmentId := range replica.Assignments {
				replicaSnap.Assignments[assignmentId] = common.Unit{}
			}
		}
		snapshot.AllShards.Set(data.ShardId(shard.ShardId), shardSnap)
	}
	// all worker/assignments
	for workerId, worker := range ss.AllWorkers {
		workerSnap := costfunc.NewWorkerSnap(workerId)
		for assignId := range worker.Assignments {
			assignmentState, ok := ss.AllAssignments[assignId]
			if !ok {
				ke := kerror.Create("AssignmentNotFound", "assignment not found").With("assignmentId", assignId).With("workerId", workerId)
				panic(ke)
			}
			workerSnap.Assignments[assignmentState.ShardId] = assignId
			assignmentSnap := costfunc.NewAssignmentSnap(assignmentState.ShardId, assignmentState.ReplicaIdx, assignId, workerId)
			snapshot.AllAssignments.Set(assignId, assignmentSnap)
		}
		snapshot.AllWorkers.Set(workerId, workerSnap)
	}
	return snapshot.CompactAndFreeze()
}
