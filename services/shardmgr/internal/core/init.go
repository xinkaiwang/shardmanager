package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/etcdprov"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

func (ss *ServiceState) Init(ctx context.Context) {
	// step 1: load ServiceInfo
	ss.ServiceInfo = ss.LoadServiceInfo(ctx)     // from /smg/config/service_info.json
	ss.ServiceConfig = ss.LoadServiceConfig(ctx) // from /smg/config/service_config.json
	// step 2: load all shard state
	ss.LoadAllShardState(ctx) // (from /smg/shard_state/ to ss.AllShards)
	// setp 3: load all worker state
	ss.LoadAllWorkerState(ctx) // (from /smg/worker_state/ to ss.AllWorkers and ss.AllAssignments)
	ss.ShadowState.InitDone()

	// step 4: load current shard plan
	currentShardPlan, currentShardPlanRevision := ss.LoadCurrentShardPlan(ctx)
	ss.syncShardPlan(ctx, currentShardPlan) // based on crrentShardPlan, update ss.AllShards, and write to etcd
	// step 5: load current worker eph
	currentWorkerEph, currentWorkerEphRevision := ss.LoadCurrentWorkerEph(ctx)
	for _, workerEph := range currentWorkerEph {
		workerFullId := data.NewWorkerFullId(data.WorkerId(workerEph.WorkerId), data.SessionId(workerEph.SessionId), ss.IsStateInMemory())
		ss.writeWorkerEphToStaging(ctx, workerFullId, workerEph)
	}
	// step 6: sync workerEph to workerState
	ss.syncEphStagingToWorkerState(ctx)

	// step 6: start listening to shard plan changes
	ss.ShardPlanWatcher = NewShardPlanWatcher(ctx, ss, currentShardPlanRevision)
	// step 7: start listening to worker eph changes
	ss.WorkerEphWatcher = NewWorkerEphWatcher(ctx, ss, currentWorkerEphRevision)

	// step 10: start runloop
	// Note: The runloop is now initialized in NewServiceState
}

func (ss *ServiceState) LoadAllShardState(ctx context.Context) {
	pathPrefix := ss.PathManager.GetShardStatePathPrefix()
	// load all from etcd
	list, _ := etcdprov.GetCurrentEtcdProvider(ctx).LoadAllByPrefix(ctx, pathPrefix)
	for _, item := range list {
		shardStateJson := smgjson.ShardStateJsonFromJson(item.Value)
		ss.ShadowState.InitShardState(shardStateJson.ShardName, shardStateJson)
		shardObj := NewShardState(shardStateJson.ShardName)

		// 设置软删除状态，使用辅助函数转换
		shardObj.LameDuck = smgjson.Int82Bool(shardStateJson.LameDuck)

		ss.AllShards[data.ShardId(shardObj.ShardId)] = shardObj
	}
}

func (ss *ServiceState) LoadAllWorkerState(ctx context.Context) {
	pathPrefix := ss.PathManager.GetWorkerStatePathPrefix()
	// load all from etcd
	list, _ := etcdprov.GetCurrentEtcdProvider(ctx).LoadAllByPrefix(ctx, pathPrefix)
	for _, item := range list {
		workerStateJson := smgjson.WorkerStateJsonFromJson(item.Value)
		WorkerState := NewWorkerStateFromJson(workerStateJson)
		workerFullId := data.NewWorkerFullId(data.WorkerId(workerStateJson.WorkerId), data.SessionId(workerStateJson.SessionId), ss.IsStateInMemory())
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
