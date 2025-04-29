package solo

import (
	"context"

	cougarEtcd "github.com/xinkaiwang/shardmanager/libs/cougar/etcdprov"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

const (
	GlobalLockPath = "/smg/global_lock"
)

type SoloMode string

const (
	// SoloModeStrict: in case we lost our etcd eph lock, we will fatal and exit
	SLM_Strict SoloMode = "strict"

	// SoloModeRobust: in case we lost our etcd eph lock, we will try to re-acquire the lock.
	// We will annunce "LockLost" only if we either 1) see someone else acquired the lock, or 2) we can not re-acquire the lock after 3 times
	SLM_Robust SoloMode = "robust"
)

// SoloManager will maintain an lock in etcd and make sure only one shardmgr is running
// in start up, we will try to acquire the lock, if we can not acquire the lock, we will error and exit.
type SoloManager struct {
	podName   string
	sessionId string
	// soloMode          SoloMode
	etcdprov          cougarEtcd.EtcdProvider
	session           cougarEtcd.EtcdSession
	ServerStartTimeMs int64
	ChLockLost        chan struct{}
}

func NewSoloManager(ctx context.Context, podName string) *SoloManager {
	solo := &SoloManager{
		podName:           podName,
		sessionId:         common.GetSessionId(),
		ServerStartTimeMs: kcommon.GetWallTimeMs(),
		etcdprov:          cougarEtcd.GetCurrentEtcdProvider(ctx),
		ChLockLost:        make(chan struct{}, 1),
	}

	solo.session = solo.etcdprov.CreateEtcdSession(ctx)
	solo.PutNode("init")
	solo.session.SetStateListener(solo)
	return solo
}

func (solo *SoloManager) PutNode(reason string) {
	node := smgjson.NewGlobalLock(solo.podName, solo.sessionId, solo.session.GetLeaseId(), solo.ServerStartTimeMs, common.GetVersion())
	node.LastUpdateTimeMs = kcommon.GetWallTimeMs()
	node.LastUpdateResason = reason
	solo.session.PutNode(GlobalLockPath, node.ToJson())
}

func (solo *SoloManager) OnStateChange(state cougarEtcd.EtcdSessionState, msg string) {
	klogging.Info(context.Background()).With("newState", state).With("reason", msg).Log("SoloManager", "etcd state change")
	if state == cougarEtcd.ESS_Disconnected {
		close(solo.ChLockLost)
	}
}

func (solo *SoloManager) Close(ctx context.Context) {
	solo.session.Close(ctx)
}
