package core

// type ServiceInfo struct {
// 	// ServiceName 是服务的名称
// 	ServiceName string

// 	// StatefulType 服务的类型 (stateless, ST_MEMORY, ST_HARD_DRIVE)
// 	StatefulType data.StatefulType

// 	DefaultHints config.ShardConfig
// }

// func NewServiceInfo(serviceName string) *ServiceInfo {
// 	return &ServiceInfo{
// 		ServiceName: serviceName,
// 	}
// }

// func (ss *ServiceState) LoadServiceInfo(ctx context.Context) *ServiceInfo {
// 	path := ss.PathManager.GetServiceInfoPath()
// 	node := etcdprov.GetCurrentEtcdProvider(ctx).Get(ctx, path)
// 	if node.Value == "" {
// 		ke := kerror.Create("ServiceInfoNotFound", "service info not found path="+path)
// 		panic(ke)
// 	}
// 	siObj := smgjson.ParseServiceInfoJson(node.Value)
// 	si := NewServiceInfo(siObj.ServiceName)
// 	// ServiceType (default softStateful)
// 	if siObj.StatefulType != nil {
// 		si.StatefulType = *siObj.StatefulType
// 	} else {
// 		si.StatefulType = data.ST_MEMORY
// 	}
// 	// MoveType (default 先启后杀)
// 	if siObj.MoveType != nil {
// 		si.DefaultHints.MovePolicy = *siObj.MoveType
// 	} else {
// 		si.DefaultHints.MovePolicy = smgjson.MP_StartBeforeKill
// 	}
// 	// MaxResplicaCount/MinResplicaCount (default 10/1)
// 	if siObj.MaxResplicaCount != nil {
// 		si.DefaultHints.MaxReplicaCount = int(*siObj.MaxResplicaCount)
// 	} else {
// 		si.DefaultHints.MaxReplicaCount = 10
// 	}
// 	if siObj.MinResplicaCount != nil {
// 		si.DefaultHints.MinReplicaCount = int(*siObj.MinResplicaCount)
// 	} else {
// 		si.DefaultHints.MinReplicaCount = 1
// 	}
// 	return si
// }
