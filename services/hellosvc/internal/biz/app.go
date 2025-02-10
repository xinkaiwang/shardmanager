package biz

import (
	"fmt"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
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
func (a *App) GetVersion() string {
	return version
}

func (a *App) Hello(name string) string {
	return fmt.Sprintf("Hello, %s!", name)
}

// HelloWithKerror 总是抛出一个 kerror
func (a *App) HelloWithKerror(name string) string {
	panic(kerror.Create("TestKerror", "this is a test kerror").
		WithErrorCode(kerror.EC_INVALID_PARAMETER).
		With("name", name))
}

// HelloWithError 总是抛出一个普通 error
func (a *App) HelloWithError(name string) string {
	panic(fmt.Errorf("this is a test error"))
}

// HelloWithPanic 总是抛出一个非错误类型的 panic
func (a *App) HelloWithPanic(name string) string {
	panic("this is a non-error panic")
}
