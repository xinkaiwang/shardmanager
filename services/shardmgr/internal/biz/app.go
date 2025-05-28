package biz

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kmetrics"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/api"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/core"
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

func (app *App) StartAppMetrics(ctx context.Context) {
	// 启动应用级别的指标收集
	klogging.Info(ctx).Log("app.StartAppMetrics", "Starting application metrics")
	// dynamic threashold
	kmetrics.AddInt64DerivedGaugeWithLabels(ctx, app.registry,
		func() int64 { return app.ss.MetricsValueDynamicThreshold.Load() },
		"dynamic_threshold",
		"Dynamic threshold for accepting ",
		map[string]string{"smg": app.ss.Name},
	)
	// worker count
	kmetrics.AddInt64DerivedGaugeWithLabels(ctx, app.registry,
		func() int64 { return app.ss.MetricsValueWorkerCount_total.Load() },
		"worker_count_total",
		"Total number of workers in the system",
		map[string]string{"smg": app.ss.Name},
	)
	kmetrics.AddInt64DerivedGaugeWithLabels(ctx, app.registry,
		func() int64 { return app.ss.MetricsValueWorkerCount_online.Load() },
		"worker_count_online",
		"Number of active workers in the system",
		map[string]string{"smg": app.ss.Name},
	)
	kmetrics.AddInt64DerivedGaugeWithLabels(ctx, app.registry,
		func() int64 { return app.ss.MetricsValueWorkerCount_offline.Load() },
		"worker_count_offline",
		"Number of active workers in the system",
		map[string]string{"smg": app.ss.Name},
	)
	kmetrics.AddInt64DerivedGaugeWithLabels(ctx, app.registry,
		func() int64 { return app.ss.MetricsValueWorkerCount_draining.Load() },
		"worker_count_draining",
		"Number of active workers in the system",
		map[string]string{"smg": app.ss.Name},
	)
	kmetrics.AddInt64DerivedGaugeWithLabels(ctx, app.registry,
		func() int64 { return app.ss.MetricsValueWorkerCount_shutdownReq.Load() },
		"worker_count_shutdownReq",
		"Number of active workers in the system",
		map[string]string{"smg": app.ss.Name},
	)
	// shard count
	kmetrics.AddInt64DerivedGaugeWithLabels(ctx, app.registry,
		func() int64 { return app.ss.MetricsValueShardCount.Load() },
		"shard_count",
		"Total number of shards in the system",
		map[string]string{"smg": app.ss.Name},
	)
	// replica count
	kmetrics.AddInt64DerivedGaugeWithLabels(ctx, app.registry,
		func() int64 { return app.ss.MetricsValueReplicaCount.Load() },
		"replica_count",
		"Total number of replicas in the system",
		map[string]string{"smg": app.ss.Name},
	)
	// assignment count
	kmetrics.AddInt64DerivedGaugeWithLabels(ctx, app.registry,
		func() int64 { return app.ss.MetricsValueAssignmentCount.Load() },
		"assignment_count",
		"Total number of assignments in the system",
		map[string]string{"smg": app.ss.Name},
	)
	// soft/hard score
	kmetrics.AddInt64DerivedGaugeWithLabels(ctx, app.registry,
		func() int64 { return app.ss.MetricsValueCurrentSoftCost.Load() },
		"cost_current_soft",
		"Current soft cost of the system",
		map[string]string{"smg": app.ss.Name},
	)
	kmetrics.AddInt64DerivedGaugeWithLabels(ctx, app.registry,
		func() int64 { return app.ss.MetricsValueCurrentHardCost.Load() },
		"cost_current_hard",
		"Current hard cost of the system",
		map[string]string{"smg": app.ss.Name},
	)
	kmetrics.AddInt64DerivedGaugeWithLabels(ctx, app.registry,
		func() int64 { return app.ss.MetricsValueFutureSoftCost.Load() },
		"cost_future_soft",
		"Future soft cost of the system",
		map[string]string{"smg": app.ss.Name},
	)
	kmetrics.AddInt64DerivedGaugeWithLabels(ctx, app.registry,
		func() int64 { return app.ss.MetricsValueFutureHardCost.Load() },
		"cost_future_hard",
		"Future hard cost of the system",
		map[string]string{"smg": app.ss.Name},
	)
}

// GetStateEvent: implement IEvent[*core.ServiceState]
type GetStateEvent struct {
	resp chan *api.GetStateResponse
}

func NewGetStateEvent() *GetStateEvent {
	return &GetStateEvent{
		resp: make(chan *api.GetStateResponse, 1),
	}
}

func (eve *GetStateEvent) GetName() string {
	return "GetStateEvent"
}

func (gse *GetStateEvent) Process(ctx context.Context, ss *core.ServiceState) {
	klogging.Info(ctx).Log("GetStateEvent", "getting state")
	workers := make([]api.WorkerVm, 0)
	shards := make([]api.ShardVm, 0)
	for _, workerState := range ss.AllWorkers {
		worker := api.WorkerVm{
			WorkerFullId: workerState.GetWorkerFullId().String(),
		}
		assignments := workerState.CollectCurrentAssignments(ss)
		for _, assignment := range assignments {
			vm := &api.AssignmentVm{
				ShardId:      assignment.ShardId,
				ReplicaIdx:   assignment.ReplicaIdx,
				WorkerFullId: assignment.WorkerFullId.String(),
				AssignmentId: assignment.AssignmentId,
				CurrentState: assignment.CurrentConfirmedState,
				TargetState:  assignment.TargetState,
			}
			worker.Assignments = append(worker.Assignments, vm)
		}
		workers = append(workers, worker)
	}

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

	gse.resp <- &api.GetStateResponse{
		Workers: workers,
		Shards:  shards,
	}
	close(gse.resp)
}
