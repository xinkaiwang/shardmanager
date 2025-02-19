package biz

import (
	"context"
	"time"

	"github.com/xinkaiwang/shardmanager/services/etcdmgr/api"
	"github.com/xinkaiwang/shardmanager/services/etcdmgr/internal/provider"
)

var (
	version = "dev" // 默认版本号，会在构建时通过 -ldflags 覆盖
)

// SetVersion 设置服务版本
func SetVersion(v string) {
	if v != "" {
		version = v
	}
}

// App 封装了 etcd 管理的业务逻辑
type App struct {
	provider provider.EtcdProvider
}

// NewApp 创建一个新的 App 实例
func NewApp() *App {
	return &App{
		provider: provider.NewDefaultEtcdProvider(context.Background()),
	}
}

// GetVersion 返回服务版本
func (app *App) GetVersion() string {
	return version
}

// Status 返回服务状态
func (app *App) Status(ctx context.Context) api.StatusResponse {
	return api.StatusResponse{
		Status:    "ok",
		Version:   app.GetVersion(),
		Timestamp: time.Now().Format(time.RFC3339),
	}
}

// Ping 返回服务 ping 响应
func (app *App) Ping(ctx context.Context) api.PingResponse {
	return api.PingResponse{
		Status:    "ok",
		Version:   app.GetVersion(),
		Timestamp: time.Now().Format(time.RFC3339),
	}
}

// GetKey 获取指定 key 的值
func (app *App) GetKey(ctx context.Context, key string) (*api.EtcdKeyResponse, error) {
	item := app.provider.Get(ctx, key)
	if item.Value == "" {
		return nil, provider.ErrKeyNotFound
	}

	return &api.EtcdKeyResponse{
		Key:     item.Key,
		Value:   item.Value,
		Version: item.ModRevision,
	}, nil
}

// PutKey 设置指定 key 的值
func (app *App) PutKey(ctx context.Context, key, value string) error {
	app.provider.Set(ctx, key, value)
	return nil
}

// DeleteKey 删除指定的 key
func (app *App) DeleteKey(ctx context.Context, key string) error {
	app.provider.Delete(ctx, key)
	return nil
}

// ListKeys 列出指定前缀的所有 key
func (app *App) ListKeys(ctx context.Context, prefix string) (*api.EtcdKeysResponse, error) {
	items := app.provider.List(ctx, prefix, 0)
	keys := make([]api.EtcdKeyResponse, 0, len(items))
	for _, item := range items {
		keys = append(keys, api.EtcdKeyResponse{
			Key:     item.Key,
			Value:   item.Value,
			Version: item.ModRevision,
		})
	}

	return &api.EtcdKeysResponse{Keys: keys}, nil
}
