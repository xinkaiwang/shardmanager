package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/etcdprov"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

type ServiceInfo struct {
	// ServiceName 是服务的名称
	ServiceName string

	// ServiceType 服务的类型 (stateless, softStateful, hardStateful)
	ServiceType smgjson.ServiceType

	DefaultHints config.ShardConfig
}

func NewServiceInfo(serviceName string) *ServiceInfo {
	return &ServiceInfo{
		ServiceName: serviceName,
	}
}

func (ss *ServiceState) LoadServiceInfo(ctx context.Context) *ServiceInfo {
	path := ss.PathManager.GetServiceInfoPath()
	node := etcdprov.GetCurrentEtcdProvider(ctx).Get(ctx, path)
	if node.Value == "" {
		ke := kerror.Create("ServiceInfoNotFound", "service info not found path="+path)
		panic(ke)
	}
	siObj := smgjson.ParseServiceInfoJson(node.Value)
	si := NewServiceInfo(siObj.ServiceName)
	// ServiceType (default softStateful)
	if siObj.ServiceType != nil {
		si.ServiceType = *siObj.ServiceType
	} else {
		si.ServiceType = smgjson.ST_SOFT_STATEFUL
	}
	// MoveType (default 先启后杀)
	if siObj.MoveType != nil {
		si.DefaultHints.MovePolicy = *siObj.MoveType
	} else {
		si.DefaultHints.MovePolicy = smgjson.MP_StartBeforeKill
	}
	// MaxResplicaCount/MinResplicaCount (default 10/1)
	if siObj.MaxResplicaCount != nil {
		si.DefaultHints.MaxReplicaCount = *siObj.MaxResplicaCount
	} else {
		si.DefaultHints.MaxReplicaCount = 10
	}
	if siObj.MinResplicaCount != nil {
		si.DefaultHints.MinReplicaCount = *siObj.MinResplicaCount
	} else {
		si.DefaultHints.MinReplicaCount = 1
	}
	return si
}
