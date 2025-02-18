package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/etcdprov"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

const (
	serviceInfoPath = "/smg/config/service_info.json"
)

type ServiceInfo struct {
	// ServiceName 是服务的名称
	ServiceName string

	// ServiceType 服务的类型 (stateless, softStateful, hardStateful)
	ServiceType smgjson.ServiceType

	DefaultHints smgjson.ShardHints
}

func NewServiceInfo(serviceName string, serviceType smgjson.ServiceType) *ServiceInfo {
	return &ServiceInfo{
		ServiceName: serviceName,
		ServiceType: serviceType,
	}
}

func LoadServiceInfo(ctx context.Context) *ServiceInfo {
	node := etcdprov.GetCurrentEtcdProvider(ctx).Get(ctx, serviceInfoPath)
	if node.Value == "" {
		ke := kerror.Create("ServiceInfoNotFound", "service info not found path="+serviceInfoPath)
		panic(ke)
	}
	siObj := smgjson.ParseServiceInfoJson(node.Value)
	si := NewServiceInfo(siObj.ServiceName, siObj.ServiceType)
	// MoveType (default 先启后杀)
	if siObj.MoveType != nil {
		si.DefaultHints.MoveType = *siObj.MoveType
	} else {
		si.DefaultHints.MoveType = smgjson.MP_StartBeforeKill
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
