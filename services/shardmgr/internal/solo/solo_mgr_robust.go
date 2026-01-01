package solo

import (
	"context"
	"sync/atomic"

	cougarEtcd "github.com/xinkaiwang/shardmanager/libs/cougar/etcdprov"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

// SoloManagerRobust will maintain an lock in etcd and make sure only one shardmgr is running.
// in start up, we will try to acquire the lock, if we can not acquire the lock, we will error and exit.

type SoloManagerRobust struct {
	podName           string
	sessionId         string
	etcdprov          cougarEtcd.EtcdProvider
	sessionWrapper    *SessionWrapper
	ServerStartTimeMs int64
	ChLockLost        chan struct{}
}

func NewSoloManagerRobust(ctx context.Context, podName string) *SoloManagerRobust {
	solo := &SoloManagerRobust{
		podName:           podName,
		sessionId:         common.GetSessionId(),
		ServerStartTimeMs: kcommon.GetWallTimeMs(),
		etcdprov:          cougarEtcd.GetCurrentEtcdProvider(ctx),
		ChLockLost:        make(chan struct{}, 1),
	}

	solo.sessionWrapper = NewSessionWrapper(ctx, solo, "init")
	go func() {
		<-solo.sessionWrapper.chClosed
		solo.sessionWrapper = nil
		solo.onSessionLost(ctx)
	}()
	return solo
}

func (solo *SoloManagerRobust) GetLckLostCh() <-chan struct{} {
	return solo.ChLockLost
}

// Close: close this lease and wait for close
func (solo *SoloManagerRobust) Close(ctx context.Context) {
	solo.sessionWrapper.Close(ctx)
}

func (solo *SoloManagerRobust) onSessionLost(ctx context.Context) {
	ke := kcommon.TryCatchRun(ctx, func() {
		solo.sessionWrapper = NewSessionWrapper(ctx, solo, "recover")
		// go solo.onSessionLost(ctx)
		go func() {
			<-solo.sessionWrapper.chClosed
			solo.sessionWrapper = nil
			solo.onSessionLost(ctx)
		}()
	})
	if ke != nil {
		klogging.Error(ctx).With("error", ke).Log("SoloManager", "failed to recover session")
		close(solo.ChLockLost)
	}
}

// implement cougarEtcd.EtcdStateListener
type SessionWrapper struct {
	parent   *SoloManagerRobust
	session  cougarEtcd.EtcdSession
	stop     atomic.Bool
	chClosed chan struct{}
}

func NewSessionWrapper(ctx context.Context, parent *SoloManagerRobust, reason string) *SessionWrapper {
	wrapper := &SessionWrapper{
		parent:   parent,
		chClosed: make(chan struct{}, 1),
	}
	wrapper.session = parent.etcdprov.CreateEtcdSession(ctx)
	klogging.Info(ctx).With("sessionId", parent.sessionId).Log("NewSessionWrapper", "session created")
	node := smgjson.NewGlobalLock(parent.podName, parent.sessionId, wrapper.session.GetLeaseId(), parent.ServerStartTimeMs, common.GetVersion())
	node.LastUpdateTimeMs = kcommon.GetWallTimeMs()
	node.LastUpdateResason = reason
	wrapper.session.PutNode(GlobalLockPath, node.ToJson())
	wrapper.session.SetStateListener(wrapper)
	return wrapper
}

func (sw *SessionWrapper) OnStateChange(state cougarEtcd.EtcdSessionState, msg string) {
	if state == cougarEtcd.ESS_Disconnected {
		// 只有在非正常关闭时才触发 session lost 恢复逻辑
		if !sw.stop.Load() {
			close(sw.chClosed)
		}
	}
	if !sw.stop.Load() {
		sw.session.Close(context.Background())
	}
}

// close session and wait for close
func (sw *SessionWrapper) Close(ctx context.Context) {
	sw.stop.Store(true)
	sw.session.Close(ctx)
	select {
	case <-sw.chClosed:
	default:
		close(sw.chClosed)
	}
}
