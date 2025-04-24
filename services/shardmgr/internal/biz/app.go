package biz

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/api"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/core"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

type App struct {
	ss *core.ServiceState
}

func NewApp(ctx context.Context) *App {
	ss := core.AssembleSsAll(ctx, "app")
	app := &App{
		ss: ss,
	}
	return app
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
