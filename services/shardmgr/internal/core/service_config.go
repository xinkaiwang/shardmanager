package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/etcdprov"
)

func (ss *ServiceState) LoadServiceConfig(ctx context.Context) *config.ServiceConfig {
	path := ss.PathManager.GetServiceConfigPath()
	node := etcdprov.GetCurrentEtcdProvider(ctx).Get(ctx, path)
	if node.Value == "" {
		// when not exists, create service_config.json file with default values
		defVal := config.ParseServiceConfigFromJson("")
		etcdprov.GetCurrentEtcdProvider(ctx).Set(ctx, path, defVal.ToServiceConfigJson().ToJson())
		return defVal
	}
	sc := config.ParseServiceConfigFromJson(node.Value)
	return sc
}

// ServiceConfigUpdateEvent: implement IEvent interface
type ServiceConfigUpdateEvent struct {
	NewCfg *config.ServiceConfig
}

func (e *ServiceConfigUpdateEvent) GetName() string {
	return "ServiceConfigUpdate"
}
func (e *ServiceConfigUpdateEvent) Execute(ctx context.Context, ss *ServiceState) {
	ss.onServiceConfigUpdate(ctx, e.NewCfg)
}

func (ss *ServiceState) onServiceConfigUpdate(ctx context.Context, config *config.ServiceConfig) {
	klogging.Info(ctx).With("newCfg", config.ToServiceConfigJson().ToJson()).Log("ServiceConfigUpdate", "service config updated")
	ss.ServiceConfig = config
}
