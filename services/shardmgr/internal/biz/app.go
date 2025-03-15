package biz

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/api"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/core"
)

type App struct {
	ss *core.ServiceState
}

func NewApp(ctx context.Context) *App {
	ss := core.NewServiceState(ctx, "app")
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

func (app *App) GetStatus(ctx context.Context) *api.GetStatusResponse {
	klogging.Info(ctx).Log("app.GetStatus", "")
	eve := NewGetStateEvent()
	app.ss.PostEvent(eve)
	return <-eve.resp
}

// GetStateEvent: implement IEvent[*core.ServiceState]
type GetStateEvent struct {
	resp chan *api.GetStatusResponse
}

func NewGetStateEvent() *GetStateEvent {
	return &GetStateEvent{
		resp: make(chan *api.GetStatusResponse, 1),
	}
}

func (eve *GetStateEvent) GetName() string {
	return "GetStateEvent"
}

func (gse *GetStateEvent) Process(ctx context.Context, ss *core.ServiceState) {
	klogging.Info(ctx).Log("GetStateEvent", "getting state")
	shards := make([]api.ShardState, 0)
	for _, shardState := range ss.AllShards {
		shards = append(shards, api.ShardState{
			ShardId: string(shardState.ShardId),
		})
	}
	workers := make([]api.WorkerState, 0)
	for _, workerState := range ss.AllWorkers {
		worker := api.WorkerState{
			WorkerFullId: workerState.GetWorkerFullId(ss).String(),
		}
		workers = append(workers, worker)
	}

	gse.resp <- &api.GetStatusResponse{
		Shards:  shards,
		Workers: workers,
	}
	close(gse.resp)
}
