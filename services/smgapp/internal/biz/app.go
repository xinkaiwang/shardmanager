package biz

import (
	"context"
	"fmt"
	"time"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/services/smgapp/api"
)

// 包级变量，用于存储构建时注入的版本信息
var (
	version = "dev" // 默认版本号，会在构建时通过 -ldflags 覆盖
)

// SetVersion 设置服务版本
func SetVersion(v string) {
	if v != "" {
		version = v
	}
}

type App struct{}

func NewApp() *App {
	return &App{}
}

// GetVersion 返回服务版本
func (app *App) GetVersion() string {
	return version
}

func (app *App) Ping(ctx context.Context) api.PingResponse {
	// klogging.Info(ctx).Log("Hello", "ping")
	return api.PingResponse{
		Status:    "ok",
		Timestamp: time.Now().Format(time.RFC3339),
		Version:   app.GetVersion(),
	}
}

func (app *App) Hello(name string) string {
	return fmt.Sprintf("Hello, %s!", name)
}

// HelloWithKerror 总是抛出一个 kerror
func (app *App) HelloWithKerror(name string) string {
	panic(kerror.Create("TestKerror", "this is a test kerror").
		WithErrorCode(kerror.EC_INVALID_PARAMETER).
		With("name", name))
}

// HelloWithError 总是抛出一个普通 error
func (app *App) HelloWithError(name string) string {
	panic(fmt.Errorf("this is a test error"))
}

// HelloWithPanic 总是抛出一个非错误类型的 panic
func (app *App) HelloWithPanic(name string) string {
	panic("this is a non-error panic")
}
