package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/cougar/cougarjson"
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
	ss.SnapshotCurrent = costfunc.NewSnapshot(ctx, ss.ServiceConfig.CostFuncCfg)
	ss.SnapshotFuture = costfunc.NewSnapshot(ctx, ss.ServiceConfig.CostFuncCfg)
	solver.GetCurrentSolverConfigProvider().OnSolverConfigChange(&ss.ServiceConfig.SolverConfig)
	ss.DynamicThreshold = NewDynamicThreshold(func() config.DynamicThresholdConfig {
		return ss.ServiceConfig.DynamicThresholdConfig
	})

	// setp 2: load all worker state
	ss.LoadAllWorkerState(ctx)             // (from /smg/worker_state/<workerFullId> to ss.AllWorkers and ss.AllAssignments)
	pilotNodes := ss.LoadAllPilotNode(ctx) // (from /smg/pilot/<workerFullId>)

	// step 3: load all shard state (note: we need to load workerStates first, because workerState will populate assignments)
	ss.LoadAllShardState(ctx) // (from /smg/shard_state/ to ss.AllShards)
	ss.LoadAllMoves(ctx)      // (from /smg/move_state/ to ss.AllMoves)
	ss.ShadowState.InitDone()
	ss.PrintAllShards(ctx)
	ss.PrintAllWorkers(ctx)
	ss.PrintAllAssignments(ctx)
	klogging.Info(ctx).Log("ServiceStateInit", "load all shard and worker state done")

	// step 3b: remove non-referenced pilot nodes
	for workerFullId := range pilotNodes {
		if _, ok := ss.AllWorkers[workerFullId]; !ok {
			klogging.Warning(ctx).With("workerFullId", workerFullId).Log("ServiceStateInit", "pilot node not found in worker state, remove it")
			ss.pilotProvider.StorePilotNode(ctx, workerFullId, nil) // remove pilot node
			delete(pilotNodes, workerFullId)
		}
	}
	for workerFullId, workerState := range ss.AllWorkers {
		newPilotNode := workerState.ToPilotNode(ctx, ss, "init")
		ss.pilotProvider.StorePilotNode(ctx, workerFullId, newPilotNode)
	}

	// step 4: load current shard plan
	currentShardPlan, currentShardPlanRevision := ss.LoadCurrentShardPlan(ctx)
	ss.stagingShardPlan = currentShardPlan // staging area, will be used in ss.syncShardPlan
	ss.digestStagingShardPlan(ctx)         // based on crrentShardPlan, update ss.AllShards, and write to etcd

	// step 5: current snapshot and future snapshot
	ss.ReCreateSnapshot(ctx, "ServiceState.Init")
	klogging.Info(ctx).Log("ServiceStateInit", "create snapshot done")

	// step 6: load current worker eph (this need to be after first create snapshot, first digest needs to update snapshot)
	currentWorkerEph, currentWorkerEphRevision := ss.LoadCurrentWorkerEph(ctx)
	ss.batchAddToStagingWorkerEph(ctx, currentWorkerEph) // staging area, will be used in ss.syncEphStagingToWorkerState
	// step 7: sync workerEph to workerState
	ss.firstDigestStagingWorkerEph(ctx)

	// step 8: start listening to shard plan changes/worker eph changes/service config changes
	klogging.Info(ctx).Log("ServiceStateInit", "start watchers")
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
	kcommon.ScheduleRun(30*1000, func() { // 30s
		ss.PostEvent(NewHousekeep30sEvent())
	})
	metricsInitAcceptEvent(ctx)
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
	// mark all empty replicas as lame duck
	for _, shardState := range ss.AllShards {
		for replicaIdx, replicaState := range shardState.Replicas {
			if len(replicaState.Assignments) == 0 && !replicaState.LameDuck {
				// replicaState.LameDuck = true // mark as lame duck
				delete(shardState.Replicas, replicaIdx) // remove replica
				klogging.Info(ctx).With("shardId", shardState.ShardId).With("replicaIdx", replicaIdx).Log("LoadAllShardState", "delete empty replica")
			}
		}
	}
}

func (ss *ServiceState) LoadAllWorkerState(ctx context.Context) {
	pathPrefix := ss.PathManager.GetWorkerStatePathPrefix()
	// load all from etcd
	list, _ := etcdprov.GetCurrentEtcdProvider(ctx).LoadAllByPrefix(ctx, pathPrefix)
	for _, item := range list {
		workerStateJson := smgjson.WorkerStateJsonFromJson(item.Value)
		workerState := NewWorkerStateFromJson(ctx, ss, workerStateJson)
		workerState.Journal(ctx, "NewWorkerStateFromJson")

		workerFullId := data.NewWorkerFullId(workerStateJson.WorkerId, workerStateJson.SessionId, data.StatefulType(workerStateJson.StatefulType))
		if common.BoolFromInt8(workerStateJson.Hat) {
			ss.ShutdownHat[workerFullId] = common.Unit{}
		}
		ss.ShadowState.InitWorkerState(workerFullId, workerStateJson)
		ss.AllWorkers[workerFullId] = workerState
		// assignments
		// if !workerState.shouldWorkerIncludeInSnapshot() {
		// 	continue
		// }
		// for assignmentId, assignement := range workerStateJson.Assignments {
		// 	ss.AllAssignments[assignmentId] = NewAssignmentState(assignmentId, assignement.ShardId, assignement.ReplicaIdx, workerFullId)
		// }
	}
}

func (ss *ServiceState) LoadAllPilotNode(ctx context.Context) map[data.WorkerFullId]*cougarjson.PilotNodeJson {
	dict := make(map[data.WorkerFullId]*cougarjson.PilotNodeJson)
	pathPrefix := ss.PathManager.GetPilotPathPrefix()
	// load all from etcd
	list, _ := etcdprov.GetCurrentEtcdProvider(ctx).LoadAllByPrefix(ctx, pathPrefix)
	for _, item := range list {
		pilotNodeJson := cougarjson.ParsePilotNodeJson(item.Value)
		if pilotNodeJson == nil {
			klogging.Fatal(ctx).With("item", item).Log("LoadAllPilotNode", "pilot node json parse failed")
			continue
		}
		workerFullId := data.WorkerFullIdParseFromString(item.Key[len(pathPrefix):])
		dict[workerFullId] = pilotNodeJson
	}
	return dict
}

func (ss *ServiceState) LoadAllMoves(ctx context.Context) {
	pathPrefix := ss.PathManager.GetMoveStatePrefix()
	// load all from etcd
	list, _ := etcdprov.GetCurrentEtcdProvider(ctx).LoadAllByPrefix(ctx, pathPrefix)
	for _, item := range list {
		moveStateJson := smgjson.MoveStateJsonParse(item.Value)
		moveState := MoveStateFromJson(moveStateJson)
		ctx2 := klogging.EmbedTraceId(ctx, "am_"+string(moveState.ProposalId))
		minion := NewActionMinion(ctx2, ss, moveState)
		ss.AllMoves[moveState.ProposalId] = minion
	}
}

func (ss *ServiceState) LoadCurrentShardPlan(ctx context.Context) ([]*smgjson.ShardLineJson, etcdprov.EtcdRevision) {
	path := ss.PathManager.GetShardPlanPath()
	item := etcdprov.GetCurrentEtcdProvider(ctx).Get(ctx, path)
	list := smgjson.ParseShardPlan(item.Value)
	return list, item.ModRevision
}

func (ss *ServiceState) ReCreateSnapshot(ctx context.Context, reason string) {
	current := ss.CreateSnapshotFromCurrentState(ctx)
	ss.SnapshotCurrent = current
	snapshotFuture := current.Clone()
	// apply all panding moves
	for _, minion := range ss.AllMoves {
		minion.moveState.ApplyRemainingActions(snapshotFuture, costfunc.AM_Relaxed)
	}
	snapshotFuture.Freeze()
	if ss.SolverGroup != nil {
		ss.SolverGroup.OnSnapshot(ctx, snapshotFuture, reason)
		klogging.Info(ctx).With("snapshot", snapshotFuture.ToShortString()).With("time", kcommon.GetWallTimeMs()).With("reason", reason).Log("ReCreateSnapshot", "SolverGroup.OnSnapshot")
	} else {
		klogging.Info(ctx).With("snapshot", snapshotFuture.ToShortString()).With("time", kcommon.GetWallTimeMs()).With("reason", reason).Log("ReCreateSnapshot", "SolverGroup is nil, skip OnSnapshot")
	}
	ss.SetSnapshotFuture(ctx, snapshotFuture, "ReCreateSnapshot")
}

func (ss *ServiceState) CreateSnapshotFromCurrentState(ctx context.Context) *costfunc.Snapshot {
	snapshot := costfunc.NewSnapshot(ctx, ss.ServiceConfig.CostFuncCfg)
	// all shards
	for _, shard := range ss.AllShards {
		shardSnap := costfunc.NewShardSnap(shard.ShardId, 0)
		shardSnap.TargetReplicaCount = shard.TargetReplicaCount
		for replicaIdx, replica := range shard.Replicas {
			replicaSnap := costfunc.NewReplicaSnap(shard.ShardId, replicaIdx)
			replicaSnap.LameDuck = replica.LameDuck
			for assignId := range replica.Assignments {
				assignmentState, ok := ss.AllAssignments[assignId]
				if !ok {
					ke := kerror.Create("AssignmentNotFound", "assignment not found").With("assignmentId", assignId).With("shardId", shard.ShardId)
					panic(ke)
				}
				if !shouldAssignIncludeInSnapshot(assignmentState, costfunc.ST_Current) {
					continue
				}
				replicaSnap.Assignments[assignId] = common.Unit{}
			}
			if replicaSnap.LameDuck || len(replicaSnap.Assignments) >= 0 {
				// Note: snapshot needs to include healthy replicas or lame duck replicas. This is because assign solver needs to know what are the available replica Idxs
				shardSnap.Replicas[replicaIdx] = replicaSnap
			}
		}
		snapshot.AllShards.Set(data.ShardId(shard.ShardId), shardSnap)
	}
	// all worker/assignments
	for workerId, worker := range ss.AllWorkers {
		if !worker.shouldWorkerIncludeInSnapshot() {
			continue
		}
		workerSnap := costfunc.NewWorkerSnap(workerId)
		workerSnap.Draining = worker.IsDaining()
		workerSnap.Offline = worker.IsOffline()
		for _, assignId := range worker.Assignments {
			assignmentState, ok := ss.AllAssignments[assignId]
			if !ok {
				ke := kerror.Create("AssignmentNotFound", "assignment not found").With("assignmentId", assignId).With("workerId", workerId)
				panic(ke)
			}
			if !shouldAssignIncludeInSnapshot(assignmentState, costfunc.ST_Current) {
				// snapshot only include those assignment in ready state
				continue
			}
			workerSnap.Assignments[assignmentState.ShardId] = assignId
			assignmentSnap := costfunc.NewAssignmentSnap(assignmentState.ShardId, assignmentState.ReplicaIdx, assignId, workerId)
			snapshot.AllAssignments.Set(assignId, assignmentSnap)
		}
		snapshot.AllWorkers.Set(workerId, workerSnap)
	}
	return snapshot.CompactAndFreeze()
}

func (workerState *WorkerState) shouldWorkerIncludeInSnapshot() bool {
	switch workerState.State {
	case data.WS_Online_healthy, data.WS_Online_shutdown_req, data.WS_Online_shutdown_permit:
		return true
	case data.WS_Offline_graceful_period, data.WS_Offline_draining_candidate, data.WS_Offline_draining_complete:
		return true
	case data.WS_Deleted, data.WS_Unknown, data.WS_Offline_dead:
		return false
	default:
		klogging.Fatal(context.Background()).With("workerState", workerState).Log("shouldWorkerIncludeInSnapshot", "unknown worker state")
	}
	return false
}

func shouldAssignIncludeInSnapshot(assignmentState *AssignmentState, snapshotType costfunc.SnapshotType) bool {
	switch snapshotType {
	case costfunc.ST_Current:
		// Note: CAS_Unknown typically means the eph is lost (we don't know what happened), in this case, we need include it in the snapshot
		// 1) we should assume the assign is still exists, otherwise assign-solver will create new assign which is not what we want,
		// 2) we should allow soft-solver to move it to another healthy worker.
		return assignmentState.CurrentConfirmedState == cougarjson.CAS_Ready || assignmentState.CurrentConfirmedState == cougarjson.CAS_Dropping || assignmentState.CurrentConfirmedState == cougarjson.CAS_Unknown
	case costfunc.ST_Future:
		return assignmentState.TargetState == cougarjson.CAS_Ready
	default:
		ke := kerror.Create("UnknownSnapshotType", "unknown snapshot type").With("snapshotType", snapshotType)
		panic(ke)
	}
}
