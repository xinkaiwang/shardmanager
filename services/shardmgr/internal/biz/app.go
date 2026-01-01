package biz

import (
	"context"
	"sort"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kmetrics"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/api"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/core"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"go.opencensus.io/metric"
)

type App struct {
	registry *metric.Registry
	ss       *core.ServiceState
}

func NewApp(ctx context.Context, name string) *App {
	ss := core.AssembleSsAll(ctx, name)
	app := &App{
		registry: metric.NewRegistry(),
		ss:       ss,
	}
	return app
}

func (app *App) GetRegistry() *metric.Registry {
	return app.registry
}

func (app *App) Ping(ctx context.Context) string {
	// klogging.Info(ctx).Log("Hello", "ping")
	ver := common.GetVersion()
	return "shardmgr:" + ver
}

func (app *App) GetStatus(ctx context.Context, req *api.GetStateRequest) *api.GetStateResponse {
	klogging.Info(ctx).Log("app.GetStatus", "")
	eve := NewGetStateEvent()
	app.ss.PostEvent(eve)
	return <-eve.resp
}

func (app *App) GetCurrentSnapshot(ctx context.Context) *costfunc.SnapshotVm {
	var snapshot *costfunc.Snapshot
	app.ss.PostActionAndWait(func(ss *core.ServiceState) {
		snapshot = ss.GetSnapshotCurrentForClone()
	}, "GetCurrentSnapshot")
	snapshotVm := snapshot.ToVm()
	return snapshotVm
}

func (app *App) GetFutureSnapshot(ctx context.Context) *costfunc.SnapshotVm {
	var snapshot *costfunc.Snapshot
	app.ss.PostActionAndWait(func(ss *core.ServiceState) {
		snapshot = ss.GetSnapshotFutureForClone(ctx)
	}, "GetFutureSnapshot")
	snapshotVm := snapshot.ToVm()
	return snapshotVm
}

func (app *App) GetShardPlan(ctx context.Context) string {
	return app.ss.ShardPlanWatcher.GetCurrentShardPlan(ctx)
}

func (app *App) WriteShardPlan(ctx context.Context, shardPlan string) {
	app.ss.ShardPlanWatcher.WriteShardPlan(ctx, shardPlan)
}

func (app *App) StartAppMetrics(ctx context.Context) {
	// 启动应用级别的指标收集
	klogging.Info(ctx).Log("app.StartAppMetrics", "Starting application metrics")
	// dynamic threashold
	kmetrics.AddInt64DerivedGaugeWithLabels(ctx, app.registry,
		func() int64 { return app.ss.MetricsValues.MetricsValueDynamicThreshold.Load() },
		"dynamic_threshold",
		"Dynamic threshold for accepting proposals",
		map[string]string{"smg": app.ss.Name},
	)
	kmetrics.AddInt64DerivedGaugeWithLabels(ctx, app.registry,
		func() int64 { return app.ss.MetricsValues.MetricsValueBestSoftScoreInQueue.Load() },
		"best_soft_score_in_queue",
		"Best soft score in the queue",
		map[string]string{"smg": app.ss.Name},
	)
	kmetrics.AddInt64DerivedGaugeWithLabels(ctx, app.registry,
		func() int64 { return app.ss.MetricsValues.MetricsValueBestHardScoreInQueue.Load() },
		"best_hard_score_in_queue",
		"Best hard score in the queue",
		map[string]string{"smg": app.ss.Name},
	)
	// worker count
	kmetrics.AddInt64DerivedGaugeWithLabels(ctx, app.registry,
		func() int64 { return app.ss.MetricsValues.MetricsValueWorkerCount_total.Load() },
		"worker_count_total",
		"Total number of workers in the system",
		map[string]string{"smg": app.ss.Name},
	)
	kmetrics.AddInt64DerivedGaugeWithLabels(ctx, app.registry,
		func() int64 { return app.ss.MetricsValues.MetricsValueWorkerCount_online.Load() },
		"worker_count_online",
		"Number of active workers in the system",
		map[string]string{"smg": app.ss.Name},
	)
	kmetrics.AddInt64DerivedGaugeWithLabels(ctx, app.registry,
		func() int64 { return app.ss.MetricsValues.MetricsValueWorkerCount_offline.Load() },
		"worker_count_offline",
		"Number of active workers in the system",
		map[string]string{"smg": app.ss.Name},
	)
	kmetrics.AddInt64DerivedGaugeWithLabels(ctx, app.registry,
		func() int64 { return app.ss.MetricsValues.MetricsValueWorkerCount_draining.Load() },
		"worker_count_draining",
		"Number of active workers in the system",
		map[string]string{"smg": app.ss.Name},
	)
	kmetrics.AddInt64DerivedGaugeWithLabels(ctx, app.registry,
		func() int64 { return app.ss.MetricsValues.MetricsValueWorkerCount_shutdownReq.Load() },
		"worker_count_shutdownReq",
		"Number of active workers in the system",
		map[string]string{"smg": app.ss.Name},
	)
	// shard count
	kmetrics.AddInt64DerivedGaugeWithLabels(ctx, app.registry,
		func() int64 { return app.ss.MetricsValues.MetricsValueShardCount.Load() },
		"shard_count",
		"Total number of shards in the system",
		map[string]string{"smg": app.ss.Name},
	)
	// replica count
	kmetrics.AddInt64DerivedGaugeWithLabels(ctx, app.registry,
		func() int64 { return app.ss.MetricsValues.MetricsValueReplicaCount.Load() },
		"replica_count",
		"Total number of replicas in the system",
		map[string]string{"smg": app.ss.Name},
	)
	// assignment count
	kmetrics.AddInt64DerivedGaugeWithLabels(ctx, app.registry,
		func() int64 { return app.ss.MetricsValues.MetricsValueAssignmentCount.Load() },
		"assignment_count",
		"Total number of assignments in the system",
		map[string]string{"smg": app.ss.Name},
	)
	// inflight move count
	kmetrics.AddInt64DerivedGaugeWithLabels(ctx, app.registry,
		func() int64 { return app.ss.MetricsValues.MetricsValueInflightMoveCount.Load() },
		"inflight_move_count",
		"Total number of inflight moves in the system",
		map[string]string{"smg": app.ss.Name},
	)
	kmetrics.AddInt64DerivedGaugeWithLabels(ctx, app.registry,
		func() int64 {
			ageMs := app.ss.MetricsValues.MetricsValueOldestMoveAgeMs.Load()
			if ageMs > 0 {
				moveStr := app.ss.MetricsValues.MetricsValueOldestMoveStr.Load().(string)
				count := app.ss.MetricsValues.MetricsValueInflightMoveCount.Load()
				klogging.Info(ctx).With("maxAge", ageMs).With("move", moveStr).With("count", count).Log("app.StartAppMetrics", "inflight move max age")
			}
			return ageMs
		},
		"oldest_move_age_ms",
		"Age of the oldest move in milliseconds",
		map[string]string{"smg": app.ss.Name},
	)

	// soft/hard score
	kmetrics.AddInt64DerivedGaugeWithLabels(ctx, app.registry,
		func() int64 { return app.ss.MetricsValues.MetricsValueCurrentSoftCost.Load() },
		"cost_current_soft",
		"Current soft cost of the system",
		map[string]string{"smg": app.ss.Name},
	)
	kmetrics.AddInt64DerivedGaugeWithLabels(ctx, app.registry,
		func() int64 { return app.ss.MetricsValues.MetricsValueCurrentHardCost.Load() },
		"cost_current_hard",
		"Current hard cost of the system",
		map[string]string{"smg": app.ss.Name},
	)
	kmetrics.AddInt64DerivedGaugeWithLabels(ctx, app.registry,
		func() int64 { return app.ss.MetricsValues.MetricsValueFutureSoftCost.Load() },
		"cost_future_soft",
		"Future soft cost of the system",
		map[string]string{"smg": app.ss.Name},
	)
	kmetrics.AddInt64DerivedGaugeWithLabels(ctx, app.registry,
		func() int64 { return app.ss.MetricsValues.MetricsValueFutureHardCost.Load() },
		"cost_future_hard",
		"Future hard cost of the system",
		map[string]string{"smg": app.ss.Name},
	)

	// SystemLimit
	kmetrics.AddInt64DerivedGaugeWithLabels(ctx, app.registry,
		func() int64 { return int64(app.ss.ServiceConfig.SystemLimit.MaxShardsCountLimit) },
		"system_limit_max_shards_count",
		"Maximum number of shards allowed in the system",
		map[string]string{"smg": app.ss.Name},
	)
	kmetrics.AddInt64DerivedGaugeWithLabels(ctx, app.registry,
		func() int64 { return int64(app.ss.ServiceConfig.SystemLimit.MaxReplicaCountLimit) },
		"system_limit_max_replica_count",
		"Maximum number of replicas allowed in the system",
		map[string]string{"smg": app.ss.Name},
	)
	kmetrics.AddInt64DerivedGaugeWithLabels(ctx, app.registry,
		func() int64 { return int64(app.ss.ServiceConfig.SystemLimit.MaxAssignmentCountLimit) },
		"system_limit_max_assignment_count",
		"Maximum number of assignments allowed in the system",
		map[string]string{"smg": app.ss.Name},
	)
	kmetrics.AddInt64DerivedGaugeWithLabels(ctx, app.registry,
		func() int64 { return int64(app.ss.ServiceConfig.SystemLimit.MaxHatCountLimit) },
		"system_limit_max_hat_count",
		"Maximum number of hats allowed in the system",
		map[string]string{"smg": app.ss.Name},
	)
	kmetrics.AddInt64DerivedGaugeWithLabels(ctx, app.registry,
		func() int64 { return int64(app.ss.ServiceConfig.SystemLimit.MaxConcurrentMoveCountLimit) },
		"system_limit_max_concurrent_move_count",
		"Maximum number of concurrent moves allowed in the system",
		map[string]string{"smg": app.ss.Name},
	)

	// runloop metrics
	kmetrics.AddInt64DerivedGaugeWithLabels(ctx, app.registry,
		func() int64 { return int64(app.ss.GetRunloopQueueLength()) },
		"runloop_queue_length",
		"Current size of runloop queue",
		map[string]string{"smg": app.ss.Name},
	)
}

// GetStateEvent: implement IEvent[*core.ServiceState]
type GetStateEvent struct {
	createTimeMs int64 // time when the event was enqueued
	resp         chan *api.GetStateResponse
}

func NewGetStateEvent() *GetStateEvent {
	return &GetStateEvent{
		createTimeMs: kcommon.GetWallTimeMs(),
		resp:         make(chan *api.GetStateResponse, 1),
	}
}

func (eve *GetStateEvent) GetCreateTimeMs() int64 {
	return eve.createTimeMs
}

func (eve *GetStateEvent) GetName() string {
	return "GetStateEvent"
}

func (gse *GetStateEvent) Process(ctx context.Context, ss *core.ServiceState) {
	klogging.Info(ctx).Log("GetStateEvent", "getting state")
	workers := make([]api.WorkerVm, 0)
	shards := make([]api.ShardVm, 0)
	for _, workerState := range ss.AllWorkers {
		if workerState.GetState() == data.WS_Deleted || workerState.GetState() == data.WS_Offline_dead || workerState.GetState() == data.WS_Offline_draining_complete {
			// skip deleted or dead workers
			continue
		}
		worker := api.WorkerVm{
			WorkerFullId:        workerState.GetWorkerFullId().String(),
			WorkerId:            string(workerState.WorkerId),
			SessionId:           string(workerState.SessionId),
			IsOffline:           common.Int8FromBool(workerState.IsOffline()),
			IsShutdownReq:       common.Int8FromBool(workerState.ShutdownRequesting),
			IsShutdownPermitted: common.Int8FromBool(workerState.GetShutdownPermited()),
			IsDraining:          common.Int8FromBool(workerState.IsDraining()),
			IsNotTarget:         common.Int8FromBool(workerState.IsNotTarget()),
		}
		if workerState.WorkerInfo != nil {
			worker.WorkerStartTimeMs = workerState.WorkerInfo.StartTimeMs
		}
		if workerState.EphNode != nil {
			worker.WorkerLastUpdateMs = workerState.EphNode.LastUpdateAtMs
		}
		assignments := workerState.CollectCurrentAssignments(ss)
		for _, assignment := range assignments {
			vm := &api.AssignmentVm{
				ShardId:      assignment.ShardId,
				ReplicaIdx:   assignment.ReplicaIdx,
				WorkerFullId: assignment.WorkerFullId.String(),
				AssignmentId: assignment.AssignmentId,
				Status:       assignment.GetStateVmString(),
			}
			worker.Assignments = append(worker.Assignments, vm)
		}
		sort.Slice(worker.Assignments, func(i, j int) bool {
			return worker.Assignments[i].ShardId < worker.Assignments[j].ShardId
		})
		workers = append(workers, worker)
	}
	// sort workers by WorkerFullId
	sort.Slice(workers, func(i, j int) bool {
		return workers[i].WorkerFullId < workers[j].WorkerFullId
	})

	for _, shardState := range ss.AllShards {
		shard := api.ShardVm{
			ShardId: shardState.ShardId,
		}
		for _, replicaStatus := range shardState.Replicas {
			replica := api.ReplicaVm{
				ReplicaIdx:  replicaStatus.ReplicaIdx,
				Assignments: make([]data.AssignmentId, 0),
			}
			for assignId := range replicaStatus.Assignments {
				replica.Assignments = append(replica.Assignments, assignId)
			}
			shard.Replicas = append(shard.Replicas, &replica)
		}
		shards = append(shards, shard)
	}
	sort.Slice(shards, func(i, j int) bool {
		return shards[i].ShardId < shards[j].ShardId
	})

	gse.resp <- &api.GetStateResponse{
		Workers: workers,
		Shards:  shards,
	}
	close(gse.resp)
}
