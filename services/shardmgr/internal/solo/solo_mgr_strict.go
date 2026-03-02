package solo

import (
	"context"

	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"

	cougarEtcd "github.com/xinkaiwang/shardmanager/libs/cougar/etcdprov"
	"log/slog"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
)

// SoloManagerStrict will maintain an lock in etcd and make sure only one shardmgr is running
// in start up, we will try to acquire the lock, if we can not acquire the lock, we will error and exit.
type SoloManagerStrict struct {
	podName   string
	sessionId string
	// soloMode          SoloMode
	etcdprov          cougarEtcd.EtcdProvider
	session           cougarEtcd.EtcdSession
	ServerStartTimeMs int64
	ChLockLost        chan struct{}
}

func NewSoloManager(ctx context.Context, podName string) *SoloManagerStrict {
	solo := &SoloManagerStrict{
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

func (solo *SoloManagerStrict) GetLckLostCh() <-chan struct{} {
	return solo.ChLockLost
}

func (solo *SoloManagerStrict) PutNode(reason string) {
	node := smgjson.NewGlobalLock(solo.podName, solo.sessionId, solo.session.GetLeaseId(), solo.ServerStartTimeMs, common.GetVersion())
	node.LastUpdateTimeMs = kcommon.GetWallTimeMs()
	node.LastUpdateResason = reason
	solo.session.PutNode(GlobalLockPath, node.ToJson())
}

func (solo *SoloManagerStrict) OnStateChange(state cougarEtcd.EtcdSessionState, msg string) {
	slog.InfoContext(context.Background(), "etcd state change", slog.String("event", "SoloManager"), slog.Any("newState", state), slog.String("reason", msg))
	if state == cougarEtcd.ESS_Disconnected {
		close(solo.ChLockLost)
	}
}

func (solo *SoloManagerStrict) Close(ctx context.Context) {
	solo.session.Close(ctx)
}
