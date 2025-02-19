package biz

import (
	"context"

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
	return ver
}
