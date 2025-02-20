package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/etcdprov"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

func (ss *ServiceState) Init(ctx context.Context) {
	// step 1: load ServiceInfo
	ss.ServiceInfo = LoadServiceInfo(ctx)
	// step 2: load all shard state
	ss.AllShards = ss.LoadAllShardState(ctx)
	// setp 3: load all worker state
	ss.AllWorkers = ss.LoadAllWorkerState(ctx)
	// step 4: load current shard plan
	currentShardPlan, currentShardPlanRevision := ss.LoadCurrentShardPlan(ctx)
	ss.syncShardPlan(ctx, currentShardPlan)
	// step 5: start listening to shard plan changes
	ss.ShardPlanWatcher = NewShardPlanWatcher(ctx, ss, currentShardPlanRevision)
	// step 6: load current worker eph
	currentWorkerEph, currentWorkerEphRevision := ss.LoadCurrentWorkerEph(ctx)
	for _, workerEph := range currentWorkerEph {
		ss.syncWorkerEph(ctx, workerEph)
	}
	// step 7: start listening to worker state changes
	ss.WorkerEphWatcher = NewWorkerEphWatcher(ctx, ss, currentWorkerEphRevision)

	// step 10: start runloop
	ss.runloop = NewRunLoop(ctx, ss)
}

func (ss *ServiceState) LoadAllShardState(ctx context.Context) map[data.ShardId]*ShardState {
	pathPrefix := "/smg/shard_state/"
	dict := map[data.ShardId]*ShardState{}
	// load all from etcd
	list, _ := etcdprov.GetCurrentEtcdProvider(ctx).LoadAllByPrefix(ctx, pathPrefix)
	for _, item := range list {
		shardStateJson := smgjson.ShardStateJsonFromJson(item.Value)
		shardObj := NewShardState(shardStateJson.ShardName)
		dict[data.ShardId(shardObj.ShardId)] = shardObj
	}
	return dict
}

func (ss *ServiceState) LoadAllWorkerState(ctx context.Context) map[data.WorkerFullId]*WorkerState {
	pathPrefix := "/smg/worker_state/"
	dict := map[data.WorkerFullId]*WorkerState{}
	// load all from etcd
	list, _ := etcdprov.GetCurrentEtcdProvider(ctx).LoadAllByPrefix(ctx, pathPrefix)
	for _, item := range list {
		shardStateJson := smgjson.WorkerStateJsonFromJson(item.Value)
		obj := NewWorkerState(shardStateJson.WorkerId, shardStateJson.SessionId)
		workerFullId := data.NewWorkerFullId(obj.WorkerId, obj.SessionId, ss.IsStateInMemory())
		dict[workerFullId] = obj
	}
	return dict
}

func (ss *ServiceState) LoadCurrentShardPlan(ctx context.Context) ([]*smgjson.ShardLine, etcdprov.EtcdRevision) {
	path := "/smg/config/shard_plan.txt"
	item := etcdprov.GetCurrentEtcdProvider(ctx).Get(ctx, path)
	list := smgjson.ParseShardPlan(item.Value, ss.ServiceInfo.DefaultHints)
	return list, item.ModRevision
}
