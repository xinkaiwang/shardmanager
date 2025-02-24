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
	ss := core.NewServiceState(ctx)
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
	app.ss.EnqueueEvent(eve)
	return <-eve.resp
}

// GetStateEvent: implement IEvent
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

func (gse *GetStateEvent) Execute(ctx context.Context, ss *core.ServiceState) {
	klogging.Info(ctx).Log("GetStateEvent", "getting state")
	list := make([]api.ShardState, 0)
	for _, shardState := range ss.AllShards {
		list = append(list, api.ShardState{
			ShardId: string(shardState.ShardId),
		})
	}
	gse.resp <- &api.GetStatusResponse{
		Shards: list,
	}
	close(gse.resp)
}
