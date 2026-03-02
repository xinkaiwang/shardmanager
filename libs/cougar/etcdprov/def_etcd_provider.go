package etcdprov

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	clientv3 "go.etcd.io/etcd/client/v3"
)

/********************** DefEtcdProvider **********************/

// DefEtcdProvider implements EtcdProvider
type DefEtcdProvider struct {
	etcdEndpoints []string
	client        *clientv3.Client
}

// NewDefEtcdProvider creates a new EtcdProvider implementation.
// It attempts to connect to etcd and panics if the initial connection fails.
func NewDefEtcdProvider(ctx context.Context) *DefEtcdProvider {
	// 从环境变量获取配置
	endpoints := getEndpointsFromEnv()
	dialTimeoutMs := getDialTimeoutMsFromEnv()

	slog.InfoContext(ctx, "creating etcd provider",
		slog.String("event", "DefEtcdProvider.Create"),
		slog.String("endpoints", strings.Join(endpoints, ",")),
		slog.Int("dialTimeoutMs", dialTimeoutMs))
	// 创建 etcd 客户端
	// Note: clientv3.New doesn't take a context for the initial dial itself,
	// but DialTimeout controls the connection attempt duration.
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: time.Duration(dialTimeoutMs) * time.Millisecond,
	})
	if err != nil {
		ke := kerror.Create("EtcdConnectError", "failed to connect to etcd").
			WithErrorCode(kerror.EC_INTERNAL_ERROR).
			With("endpoints", strings.Join(endpoints, ",")).
			With("error", err.Error())
		// Panic if connection fails
		panic(ke)
	}
	slog.InfoContext(ctx, "etcd client created successfully",
		slog.String("event", "DefEtcdProvider.ClientCreated"),
		slog.String("endpoints", strings.Join(endpoints, ",")))
	// 创建 etcd 客户端成功，返回 DefEtcdProvider 实例
	return &DefEtcdProvider{
		etcdEndpoints: endpoints,
		client:        cli,
	}
}

// CreateEtcdSession creates a new EtcdSession.
func (p *DefEtcdProvider) CreateEtcdSession(ctx context.Context) EtcdSession {
	return NewDefEtcdSession(p, ctx)
}

// DefEtcdSession implements EtcdSession
// Panics if lease cannot be granted within etcdTimeoutMs.
func NewDefEtcdSession(parent *DefEtcdProvider, ctx context.Context) *DefEtcdSession { // ctx here is mainly for logging in this func
	slog.InfoContext(ctx, "creating new etcd session",
		slog.String("event", "DefEtcdSession.Create"),
		slog.Int("etcdLeaseTimeoutMs", etcdLeaseTimeoutMs))

	// Create a context with timeout specifically for the Grant operation
	grantCtx, cancel := context.WithTimeout(ctx, time.Duration(etcdTimeoutMs)*time.Millisecond)
	defer cancel() // Ensure context is cancelled even if Grant panics or returns early

	startTime := kcommon.GetWallTimeMs()
	lease, err := parent.client.Grant(grantCtx, int64(etcdLeaseTimeoutMs/1000))
	elapsedMs := kcommon.GetWallTimeMs() - startTime
	if err != nil {
		ke := kerror.Wrap(err, "EtcdGrantError", "failed to grant lease", false).
			With("endpoints", strings.Join(parent.etcdEndpoints, ",")).
			With("elapsedMs", elapsedMs).
			With("timeoutMs", etcdTimeoutMs) // Add timeout info to error
		// Panic if lease grant fails (or times out)
		panic(ke)
	}
	slog.InfoContext(ctx, "lease granted successfully",
		slog.String("event", "DefEtcdSession.LeaseGranted"),
		slog.Int64("leaseId", int64(lease.ID)),
		slog.Int64("elapsedMs", elapsedMs))
	sessionId := strconv.FormatInt(int64(lease.ID), 10)
	// Create a context for keepalive that can be cancelled independently
	keepAliveCtx, keepAliveCancel := context.WithCancel(context.Background())

	session := &DefEtcdSession{
		sessionId:       sessionId,
		state:           ESS_Connected, // lease already connected
		parent:          parent,
		lease:           lease.ID,
		chClosed:        make(chan struct{}),
		keepAliveCancel: keepAliveCancel,
	}

	// Start keepalive in a separate goroutine
	go session.keepalive(keepAliveCtx)

	return session
}

type DefEtcdSession struct {
	sessionId string
	state     EtcdSessionState
	parent    *DefEtcdProvider
	lease     clientv3.LeaseID
	listener  EtcdStateListener

	mu              sync.RWMutex // Mutex to protect state and listener
	keepAliveCancel context.CancelFunc

	closeOnce   sync.Once // To ensure Close actions run only once
	closeReason string    // Reason for session closure, if needed
	chClosed    chan struct{}
}

func (session *DefEtcdSession) WaitForSessionClose() string {
	// Wait for the session to close and return the reason
	<-session.chClosed
	return session.closeReason
}

func (session *DefEtcdSession) keepalive(ctx context.Context) {
	slog.InfoContext(ctx, "keepalive starting",
		slog.String("event", "EtcdSession.KeepaliveStart"),
		slog.String("sessionId", session.sessionId))
	// 保持租约的续约
	keepAliveCh, err := session.parent.client.KeepAlive(ctx, session.lease)
	if err != nil {
		ke := kerror.Wrap(err, "EtcdKeepAliveError", "KeepAlive failed initially", false)
		slog.ErrorContext(ctx, "keepalive initial error",
			slog.String("event", "EtcdSession.KeepaliveInitError"),
			slog.String("sessionId", session.sessionId),
			slog.Any("error", ke))
		// Set state to disconnected because keepalive could not start
		session.setState(ESS_Disconnected, "keepalive initial error: "+err.Error())
		return // Exit goroutine
	}

	// Successfully started keepalive, set state to connected
	session.setState(ESS_Connected, "keepalive started successfully")

	stop := false
	var reason string
	for !stop {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "keepalive context cancelled",
				slog.String("event", "EtcdSession.KeepaliveCancelled"),
				slog.String("sessionId", session.sessionId))
			stop = true
			reason = "keepaliveContextCancelled"
			session.setState(ESS_Disconnected, "keepalive context cancelled")
			continue
		case ka, ok := <-keepAliveCh:
			if !ok { // Check if channel is closed
				slog.InfoContext(ctx, "keepalive channel closed",
					slog.String("event", "EtcdSession.KeepaliveChannelClosed"),
					slog.String("sessionId", session.sessionId))
				stop = true
				reason = "LeaseLost"
				// keepalive channel closed, means the lease is expired or revoked
				session.setState(ESS_Disconnected, "keepalive channel closed, lease lost")
				continue
			}
			// Log successful keepalive at Debug level to avoid flooding
			slog.DebugContext(ctx, "keepalive success",
				slog.String("event", "EtcdSession.KeepaliveSuccess"),
				slog.Int64("lease", int64(ka.ID)),
				slog.String("sessionId", session.sessionId),
				slog.Int64("ttl", ka.TTL))
			// Ensure state is connected after a successful keepalive (might have been reconnecting)
			session.mu.RLock()
			currentState := session.state
			session.mu.RUnlock()
			if currentState != ESS_Connected {
				session.setState(ESS_Connected, "keepalive successful")
			}
		}
	}
	slog.InfoContext(ctx, "keepalive loop exited",
		slog.String("event", "EtcdSession.KeepaliveExit"),
		slog.String("sessionId", session.sessionId))

	// Attempt to revoke the lease upon exiting keepalive loop, ignore errors
	revokeCtx, cancel := context.WithTimeout(context.Background(), time.Duration(etcdTimeoutMs)*time.Millisecond)
	defer cancel()
	_, _ = session.parent.client.Revoke(revokeCtx, session.lease)

	// Ensure the closed channel is signaled when keepalive exits
	session.closeOnce.Do(func() {
		session.closeReason = reason
		close(session.chClosed)
		slog.InfoContext(context.Background(), "closed channel signaled by keepalive exit",
			slog.String("event", "EtcdSession.ClosedByKeepalive"),
			slog.String("sessionId", session.sessionId))
	})
}

func (session *DefEtcdSession) setState(state EtcdSessionState, message string) {
	session.mu.Lock()
	defer session.mu.Unlock()

	if session.state == state {
		return // Avoid redundant state changes and notifications
	}

	oldState := session.state
	session.state = state
	slog.InfoContext(context.Background(), "session state changed",
		slog.String("event", "EtcdSessionStateChange"),
		slog.String("sessionId", session.sessionId),
		slog.Any("oldState", oldState),
		slog.Any("newState", state),
		slog.String("message", message))

	if session.listener != nil {
		// Call listener in a separate goroutine to avoid blocking the state change
		go func(listener EtcdStateListener, state EtcdSessionState, msg string) {
			listener.OnStateChange(state, msg)
		}(session.listener, state, message)
	}
}

func (session *DefEtcdSession) GetLeaseId() int64 {
	return int64(session.lease)
}

// DeleteNode implements EtcdSession.
func (session *DefEtcdSession) DeleteNode(key string) {
	// Check session state first
	if session.GetCurrentState() != ESS_Connected {
		panic(kerror.Create("EtcdSessionError", "session not connected").With("sessionId", session.sessionId).With("state", session.state))
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(etcdTimeoutMs)*time.Millisecond)
	defer cancel()

	_, err := session.parent.client.Delete(ctx, key)
	if err != nil {
		ke := kerror.Wrap(err, "EtcdDeleteError", "failed to delete node", false).With("sessionId", session.sessionId).With("key", key)
		panic(ke)
	}
}

// GetCurrentState implements EtcdSession.
func (session *DefEtcdSession) GetCurrentState() EtcdSessionState {
	session.mu.RLock()
	defer session.mu.RUnlock()
	return session.state
}

// PutNode implements EtcdSession.
func (session *DefEtcdSession) PutNode(key string, value string) {
	// Check session state first
	if session.GetCurrentState() != ESS_Connected {
		panic(kerror.Create("EtcdSessionError", "session not connected").With("state", session.state).With("sessionId", session.sessionId))
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(etcdTimeoutMs)*time.Millisecond)
	defer cancel()

	slog.DebugContext(ctx, "写入节点",
		slog.String("event", "PutNode"),
		slog.String("key", key),
		slog.String("value", value),
		slog.Int64("lease", int64(session.lease)),
		slog.String("sessionId", session.sessionId))

	// 尝试两种情况：
	// 1. 键不存在
	// 2. 键存在且由当前租约创建
	txn := session.parent.client.Txn(ctx)
	txn = txn.If(clientv3.Compare(clientv3.CreateRevision(key), "=", 0)).
		Then(clientv3.OpPut(key, value, clientv3.WithLease(session.lease))).
		Else(
			// 如果键存在，检查是否由当前租约创建
			clientv3.OpTxn(
				[]clientv3.Cmp{clientv3.Compare(clientv3.LeaseValue(key), "=", session.lease)},
				[]clientv3.Op{clientv3.OpPut(key, value, clientv3.WithLease(session.lease))},
				nil,
			),
		)

	// 提交事务
	txnResp, err := txn.Commit()
	if err != nil {
		ke := kerror.Wrap(err, "EtcdPutError", "failed to put node", false).With("key", key).With("value", value).With("sessionId", session.sessionId)
		panic(ke)
	}

	// 检查事务是否成功
	if !txnResp.Succeeded {
		// 如果外层事务失败，检查内层事务的结果
		if len(txnResp.Responses) > 0 && txnResp.Responses[0].Response != nil {
			if resp := txnResp.Responses[0].GetResponseTxn(); resp != nil && resp.Succeeded {
				// 内层事务成功，说明键已存在且由当前租约创建
				return
			}
		}
		ke := kerror.Create("EtcdPutError", "key exists and is not under my lease").With("key", key).With("sessionId", session.sessionId)
		panic(ke)
	}
}

// SetStateListener implements EtcdSession.
func (session *DefEtcdSession) SetStateListener(listener EtcdStateListener) {
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.listener != nil && listener != nil {
		// Allow replacing the listener, but log a warning
		slog.InfoContext(context.Background(), "replacing existing state listener",
			slog.String("event", "EtcdSession.ReplaceListener"),
			slog.String("sessionId", session.sessionId))
	}
	session.listener = listener
}

// Close implements EtcdSession.
func (session *DefEtcdSession) Close(ctx context.Context) {
	slog.InfoContext(context.Background(), "closing session explicitly",
		slog.String("event", "EtcdSession.Close"),
		slog.String("sessionId", session.sessionId))

	// Use sync.Once to ensure cleanup happens only once
	session.closeOnce.Do(func() {
		slog.InfoContext(context.Background(), "running close actions",
			slog.String("event", "EtcdSession.CloseActions"),
			slog.String("sessionId", session.sessionId))
		// Cancel the keepalive context first
		if session.keepAliveCancel != nil {
			session.keepAliveCancel()
		}

		// 显式撤销租约
		ctx, cancel := context.WithTimeout(ctx, time.Duration(etcdTimeoutMs)*time.Millisecond)
		defer cancel()
		_, err := session.parent.client.Revoke(ctx, session.lease)
		if err != nil {
			slog.InfoContext(ctx, "failed to revoke lease",
				slog.String("event", "EtcdSession.RevokeFailed"),
				slog.Any("error", err),
				slog.String("sessionId", session.sessionId),
				slog.Int64("lease", int64(session.lease)))
		} else {
			slog.InfoContext(ctx, "lease revoked successfully",
				slog.String("event", "EtcdSession.RevokeSuccess"),
				slog.String("sessionId", session.sessionId),
				slog.Int64("lease", int64(session.lease)))
		}

		// Signal watchers to close by closing the channel
		close(session.chClosed)
	})
}

func (session *DefEtcdSession) WatchByPrefix(ctx context.Context, pathPrefix string, revision EtcdRevision) chan EtcdKvItem {
	// 监听 etcd 的前缀
	ch := make(chan EtcdKvItem)
	go func() {
		defer close(ch)
		// Use a watch context that is separate or linked appropriately
		// Using session.parent.client ensures we use the main client connection
		watchCtx := ctx // Or derive from a background context if needed independent lifetime

		var watchStartRev int64
		if revision > 0 {
			watchStartRev = int64(revision)
		} else {
			// Get current revision if none provided
			getCtx, cancel := context.WithTimeout(context.Background(), time.Duration(etcdTimeoutMs)*time.Millisecond)
			resp, err := session.parent.client.Get(getCtx, pathPrefix, clientv3.WithPrefix(), clientv3.WithLimit(1))
			cancel()
			if err == nil && resp.Header != nil {
				watchStartRev = resp.Header.Revision
			} else {
				slog.InfoContext(watchCtx, "failed to get current revision for watch, starting from 0",
					slog.String("event", "EtcdWatch.GetRevisionFailed"),
					slog.Any("error", err),
					slog.String("prefix", pathPrefix),
					slog.String("sessionId", session.sessionId))
				watchStartRev = 0 // Start from beginning if get fails
			}
		}

		rch := session.parent.client.Watch(watchCtx, pathPrefix, clientv3.WithPrefix(), clientv3.WithRev(watchStartRev))
		stop := false
		var stopReason string
		for !stop {
			select {
			case <-ctx.Done():
				slog.InfoContext(watchCtx, "watch context cancelled",
					slog.String("event", "EtcdWatch.Cancelled"),
					slog.String("prefix", pathPrefix),
					slog.String("sessionId", session.sessionId))
				stop = true
				stopReason = "watch context cancelled"
				continue
			case <-session.chClosed:
				slog.InfoContext(watchCtx, "session closed, exiting watch loop",
					slog.String("event", "EtcdWatch.SessionClosed"),
					slog.String("prefix", pathPrefix),
					slog.String("sessionId", session.sessionId))
				stop = true
				stopReason = "session closed"
				continue
			case wr := <-rch:
				if wr.Err() != nil {
					slog.ErrorContext(watchCtx, "watch channel received an error",
						slog.String("event", "EtcdWatchError"),
						slog.Any("error", wr.Err()),
						slog.String("prefix", pathPrefix),
						slog.String("sessionId", session.sessionId))
					// Optionally, notify listener or attempt to restart watch based on error type
					stop = true // Exit the loop on watch error
					stopReason = "watch error"
					continue
				}
				for _, ev := range wr.Events {
					if ev.Type == clientv3.EventTypeDelete {
						ch <- EtcdKvItem{
							Key:         string(ev.Kv.Key),
							Value:       "",
							ModRevision: EtcdRevision(ev.Kv.ModRevision),
						}
					} else {
						ch <- EtcdKvItem{
							Key:         string(ev.Kv.Key),
							Value:       string(ev.Kv.Value),
							ModRevision: EtcdRevision(ev.Kv.ModRevision),
						}
					}
				}
			}
		}
		slog.InfoContext(watchCtx, "watch loop exited",
			slog.String("event", "EtcdWatch.Exit"),
			slog.String("prefix", pathPrefix),
			slog.String("sessionId", session.sessionId),
			slog.String("stopReason", stopReason))
	}()
	return ch
}
