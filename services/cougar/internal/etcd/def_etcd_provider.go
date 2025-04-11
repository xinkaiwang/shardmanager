package etcd

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	clientv3 "go.etcd.io/etcd/client/v3"
)

/********************** DefEtcdProvider **********************/

// DefEtcdProvider implements EtcdProvider
type DefEtcdProvider struct {
	etcdEndpoints []string
	client        *clientv3.Client
}

func NewDefEtcdProvider() *DefEtcdProvider {
	// 从环境变量获取配置
	endpoints := getEndpointsFromEnv()
	dialTimeoutMs := getDialTimeoutMsFromEnv()

	// 创建 etcd 客户端
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: time.Duration(dialTimeoutMs) * time.Millisecond,
	})
	if err != nil {
		ke := kerror.Create("EtcdConnectError", "failed to connect to etcd").
			WithErrorCode(kerror.EC_INTERNAL_ERROR).
			With("endpoints", strings.Join(endpoints, ",")).
			With("error", err.Error())
		panic(ke)
	}
	return &DefEtcdProvider{
		etcdEndpoints: endpoints,
		client:        cli,
	}
}

func (p *DefEtcdProvider) CreateEtcdSession(ctx context.Context) EtcdSession {
	return NewDefEtcdSession(p, ctx)
}

// DefEtcdSession implements EtcdSession
func NewDefEtcdSession(parent *DefEtcdProvider, ctx context.Context) *DefEtcdSession {
	lease, err := parent.client.Grant(ctx, int64(etcdLeaseTimeoutMs/1000))
	if err != nil {
		ke := kerror.Wrap(err, "EtcdGrantError", "failed to grant lease", false).With("endpoints", strings.Join(parent.etcdEndpoints, ","))
		panic(ke) // Keep panic here, as session creation failed fundamentally
	}

	sessionId := strconv.FormatInt(int64(lease.ID), 10)
	// Create a context for keepalive that can be cancelled independently
	keepAliveCtx, keepAliveCancel := context.WithCancel(context.Background())

	session := &DefEtcdSession{
		sessionId:       sessionId,
		state:           ESS_Connecting, // Start as connecting
		parent:          parent,
		lease:           lease.ID,
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
}

func (session *DefEtcdSession) keepalive(ctx context.Context) {
	klogging.Info(ctx).With("sessionId", session.sessionId).Log("EtcdSession", "keepalive starting")
	// 保持租约的续约
	keepAliveCh, err := session.parent.client.KeepAlive(ctx, session.lease)
	if err != nil {
		klogging.Error(ctx).WithError(kerror.Wrap(err, "EtcdKeepAliveError", "KeepAlive failed initially", false)).
			With("sessionId", session.sessionId).
			Log("EtcdSession", "keepalive initial error")
		// Set state to disconnected because keepalive could not start
		session.setState(ESS_Disconnected, "keepalive initial error: "+err.Error())
		return // Exit goroutine
	}

	// Successfully started keepalive, set state to connected
	session.setState(ESS_Connected, "keepalive started successfully")

	stop := false
	for !stop {
		select {
		case <-ctx.Done():
			klogging.Info(ctx).With("sessionId", session.sessionId).Log("EtcdSession", "keepalive context cancelled")
			stop = true
			session.setState(ESS_Disconnected, "keepalive context cancelled")
			continue
		case ka, ok := <-keepAliveCh:
			if !ok { // Check if channel is closed
				klogging.Info(ctx).With("sessionId", session.sessionId).Log("EtcdSession", "keepalive channel closed")
				stop = true
				// keepalive channel closed, means the lease is expired or revoked
				session.setState(ESS_Disconnected, "keepalive channel closed, lease lost")
				continue
			}
			// Log successful keepalive at Debug level to avoid flooding
			klogging.Debug(ctx).With("lease", ka.ID).With("sessionId", session.sessionId).With("ttl", ka.TTL).Log("EtcdSession", "keepalive success")
			// Ensure state is connected after a successful keepalive (might have been reconnecting)
			session.mu.RLock()
			currentState := session.state
			session.mu.RUnlock()
			if currentState != ESS_Connected {
				session.setState(ESS_Connected, "keepalive successful")
			}
		}
	}
	klogging.Info(ctx).With("sessionId", session.sessionId).Log("EtcdSession", "keepalive loop exited")

	// Attempt to revoke the lease upon exiting keepalive loop, ignore errors
	revokeCtx, cancel := context.WithTimeout(context.Background(), time.Duration(etcdTimeoutMs)*time.Millisecond)
	defer cancel()
	_, _ = session.parent.client.Revoke(revokeCtx, session.lease)
}

func (session *DefEtcdSession) setState(state EtcdSessionState, message string) {
	session.mu.Lock()
	defer session.mu.Unlock()

	if session.state == state {
		return // Avoid redundant state changes and notifications
	}

	oldState := session.state
	session.state = state
	klogging.Info(context.Background()).
		With("sessionId", session.sessionId).
		With("oldState", oldState).
		With("newState", state).
		With("message", message).
		Log("EtcdSessionStateChange", "Session state changed")

	if session.listener != nil {
		// Call listener in a separate goroutine to avoid blocking the state change
		go func(listener EtcdStateListener, state EtcdSessionState, msg string) {
			listener.OnStateChange(state, msg)
		}(session.listener, state, message)
	}
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

	klogging.Debug(ctx).With("key", key).With("value", value).With("lease", session.lease).With("sessionId", session.sessionId).Log("PutNode", "写入节点")
	_, err := session.parent.client.Put(ctx, key, value, clientv3.WithLease(session.lease))
	if err != nil {
		ke := kerror.Wrap(err, "EtcdPutError", "failed to put node", false).With("key", key).With("value", value).With("sessionId", session.sessionId)
		panic(ke)
	}
}

// SetStateListener implements EtcdSession.
func (session *DefEtcdSession) SetStateListener(listener EtcdStateListener) {
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.listener != nil && listener != nil {
		// Allow replacing the listener, but log a warning
		klogging.Info(context.Background()).With("sessionId", session.sessionId).Log("EtcdSession", "Replacing existing state listener")
	}
	session.listener = listener
}

// Close implements EtcdSession.
func (session *DefEtcdSession) Close() {
	klogging.Info(context.Background()).With("sessionId", session.sessionId).Log("EtcdSession", "Closing session")
	if session.keepAliveCancel != nil {
		session.keepAliveCancel() // This will stop the keepalive goroutine
	}
	// Note: Revoke is attempted inside the keepalive loop upon exit.
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
				klogging.Info(watchCtx).WithError(err).With("prefix", pathPrefix).With("sessionId", session.sessionId).Log("EtcdWatch", "Failed to get current revision for watch, starting from 0")
				watchStartRev = 0 // Start from beginning if get fails
			}
		}

		rch := session.parent.client.Watch(watchCtx, pathPrefix, clientv3.WithPrefix(), clientv3.WithRev(watchStartRev))
		for wr := range rch {
			if wr.Err() != nil {
				klogging.Error(watchCtx).WithError(wr.Err()).With("prefix", pathPrefix).With("sessionId", session.sessionId).Log("EtcdWatchError", "Watch channel received an error")
				// Optionally, notify listener or attempt to restart watch based on error type
				break // Exit the loop on watch error
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
		klogging.Info(watchCtx).With("prefix", pathPrefix).With("sessionId", session.sessionId).Log("EtcdWatch", "Watch loop exited")
	}()
	return ch
}
