package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kmetrics"
	"github.com/xinkaiwang/shardmanager/libs/xklib/krunloop"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/etcdprov"
)

var (
	configUpdateMetrics = kmetrics.CreateKmetric(context.Background(), "config_update", "service config update count", []string{"type"}).CountOnly()
)

func (ss *ServiceState) LoadServiceConfig(ctx context.Context) (*config.ServiceConfig, etcdprov.EtcdRevision) {
	path := ss.PathManager.GetServiceConfigPath()
	node := etcdprov.GetCurrentEtcdProvider(ctx).Get(ctx, path)
	if node.Value == "" {
		// when not exists, create service_config.json file with default values
		defVal := config.ParseServiceConfigFromJson("")
		etcdprov.GetCurrentEtcdProvider(ctx).Set(ctx, path, defVal.ToServiceConfigJson().ToJson())
		node = etcdprov.GetCurrentEtcdProvider(ctx).Get(ctx, path) // read the value again
	}
	klogging.Info(ctx).With("path", path).With("value", node.Value).With("revision", node.ModRevision).Log("LoadServiceConfig", "读取服务配置")
	sc := config.ParseServiceConfigFromJson(node.Value)
	return sc, node.ModRevision
}

// ServiceConfigUpdateEvent: implement IEvent interface
type ServiceConfigUpdateEvent struct {
	createTimeMs int64 // time when the event was created
	Ctx          context.Context
	Parent       *ServiceConfigWatcher
	NewCfg       *config.ServiceConfig
}

func NewServiceConfigUpdateEvent(ctx context.Context, parent *ServiceConfigWatcher, newCfg *config.ServiceConfig) *ServiceConfigUpdateEvent {
	return &ServiceConfigUpdateEvent{
		createTimeMs: kcommon.GetWallTimeMs(),
		Ctx:          ctx,
		Parent:       parent,
		NewCfg:       newCfg,
	}
}

func (e *ServiceConfigUpdateEvent) GetCreateTimeMs() int64 {
	return e.createTimeMs
}
func (e *ServiceConfigUpdateEvent) GetName() string {
	return "ServiceConfigUpdate"
}

func (e *ServiceConfigUpdateEvent) Process(ctx context.Context, ss *ServiceState) {
	ss.onServiceConfigUpdate(e.Ctx, e.NewCfg)
}

func (ss *ServiceState) onServiceConfigUpdate(ctx context.Context, config *config.ServiceConfig) {
	klogging.Info(ctx).With("newCfg", config.ToServiceConfigJson().ToJson()).Log("ServiceConfigUpdate", "service config updated")
	oldConfig := ss.ServiceConfig
	ss.ServiceConfig = config
	if oldConfig.ShardConfig != config.ShardConfig {
		for _, listener := range ss.ServiceConfigWatcher.ShardConfigListener {
			listener(&config.ShardConfig)
		}
		configUpdateMetrics.GetTimeSequence(ctx, "shard_config").Add(1)
	}
	if oldConfig.WorkerConfig != config.WorkerConfig {
		for _, listener := range ss.ServiceConfigWatcher.WorkerConfigListener {
			listener(&config.WorkerConfig)
		}
		configUpdateMetrics.GetTimeSequence(ctx, "worker_config").Add(1)
	}
	if oldConfig.SystemLimit != config.SystemLimit {
		for _, listener := range ss.ServiceConfigWatcher.SystemLimitConfigListener {
			listener(&config.SystemLimit)
		}
		configUpdateMetrics.GetTimeSequence(ctx, "system_limit").Add(1)
	}
	if oldConfig.CostFuncCfg != config.CostFuncCfg {
		for _, listener := range ss.ServiceConfigWatcher.CostfuncConfigListener {
			listener(&config.CostFuncCfg)
		}
		ss.reCreateSnapshotBatchManager.TryScheduleInternal(ctx, "cost_func_config_update")
		configUpdateMetrics.GetTimeSequence(ctx, "cost_func").Add(1)
	}
	if oldConfig.SolverConfig != config.SolverConfig {
		for _, listener := range ss.ServiceConfigWatcher.SolverConfigListener {
			listener(&config.SolverConfig)
		}
		configUpdateMetrics.GetTimeSequence(ctx, "solver_config").Add(1)
	}
	if oldConfig.DynamicThresholdConfig != config.DynamicThresholdConfig {
		for _, listener := range ss.ServiceConfigWatcher.DynamicThresholdConfigListener {
			listener(&config.DynamicThresholdConfig)
		}
		configUpdateMetrics.GetTimeSequence(ctx, "dynamic_threshold").Add(1)
	}
}

type ServiceConfigWatcher struct {
	parent                         krunloop.EventPoster[*ServiceState]
	ch                             chan etcdprov.EtcdKvItem
	ShardConfigListener            []func(*config.ShardConfig)
	WorkerConfigListener           []func(*config.WorkerConfig)
	SystemLimitConfigListener      []func(*config.SystemLimitConfig)
	CostfuncConfigListener         []func(*config.CostfuncConfig)
	SolverConfigListener           []func(*config.SolverConfig)
	DynamicThresholdConfigListener []func(*config.DynamicThresholdConfig)
}

func NewServiceConfigWatcher(ctx context.Context, parent *ServiceState, currentServiceConfigRevision etcdprov.EtcdRevision) *ServiceConfigWatcher {
	watcher := &ServiceConfigWatcher{
		parent: parent,
	}
	path := parent.PathManager.GetServiceConfigPath()
	klogging.Debug(ctx).With("path", path).With("revision", currentServiceConfigRevision).Log("ServiceConfigWatcher", "创建服务配置观察者")
	watcher.ch = etcdprov.GetCurrentEtcdProvider(ctx).WatchByPrefix(ctx, path, currentServiceConfigRevision)
	go watcher.Run(ctx)
	watcher.touchAll(ctx)
	return watcher
}

func (w *ServiceConfigWatcher) Run(ctx context.Context) {
	klogging.Info(ctx).Log("ServiceConfigWatcherRun", "started")
	stop := false
	for !stop {
		select {
		case <-ctx.Done():
			klogging.Info(ctx).Log("ServiceConfigWatcherExit", "exit")
			return
		case kvItem, ok := <-w.ch:
			traceId := kcommon.NewTraceId(ctx, "ServiceConfigWatcher", 6)
			ctx2 := klogging.EmbedTraceId(ctx, traceId)
			if !ok {
				klogging.Info(ctx2).Log("ServiceConfigWatcherExit", "exit")
				stop = true
				continue
			}
			if kvItem.Value == "" {
				// this is a delete event
				klogging.Info(ctx2).With("path", kvItem.Key).Log("ServiceConfigWatcher", "观察到服务配置已删除") // this should not happen, we ignore it
				continue
			}
			// this is a add or update event
			klogging.Info(ctx2).With("serviceConfig", kvItem.Value).With("revision", kvItem.ModRevision).Log("ServiceConfigWatcher", "观察到服务配置已更新")
			cfg := config.ParseServiceConfigFromJson(kvItem.Value)
			w.parent.PostEvent(NewServiceConfigUpdateEvent(ctx, w, cfg))
		}
	}
}

func (w *ServiceConfigWatcher) touchAll(ctx context.Context) {
	configUpdateMetrics.GetTimeSequence(ctx, "shard_config").Touch()
	configUpdateMetrics.GetTimeSequence(ctx, "worker_config").Touch()
	configUpdateMetrics.GetTimeSequence(ctx, "system_limit").Touch()
	configUpdateMetrics.GetTimeSequence(ctx, "cost_func").Touch()
	configUpdateMetrics.GetTimeSequence(ctx, "solver_config").Touch()
	configUpdateMetrics.GetTimeSequence(ctx, "dynamic_threshold").Touch()
}
