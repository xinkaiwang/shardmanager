package etcdprov

import (
	"context"
	"os"
	"strings"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
)

var (
	etcdTimeoutMs      = kcommon.GetEnvInt("ETCD_TIMEOUT_MS", 3*1000)
	etcdLeaseTimeoutMs = kcommon.GetEnvInt("ETCD_LEASE_TIMEOUT_MS", 15*1000)

	currentEtcdProvider EtcdProvider
)

func GetCurrentEtcdProvider(ctx context.Context) EtcdProvider {
	if currentEtcdProvider == nil {
		currentEtcdProvider = NewDefEtcdProvider(ctx)
	}
	return currentEtcdProvider
}

type EtcdSessionState string

const (
	ESS_Unknown      EtcdSessionState = "unknown"
	ESS_Connecting   EtcdSessionState = "connecting"
	ESS_Connected    EtcdSessionState = "connected"
	ESS_Reconnecting EtcdSessionState = "reconnecting"
	ESS_Disconnected EtcdSessionState = "disconnected"
)

// EtcdStateListener 定义了 etcd 状态监听器的接口
type EtcdStateListener interface {
	OnStateChange(state EtcdSessionState, msg string)
}

type EtcdSession interface {
	GetLeaseId() int64
	PutNode(key string, value string)
	DeleteNode(key string)
	SetStateListener(listener EtcdStateListener)
	GetCurrentState() EtcdSessionState
	WatchByPrefix(ctx context.Context, pathPrefix string, revision EtcdRevision) chan EtcdKvItem
	// Close 关闭会话，停止租约续约并释放资源
	Close(ctx context.Context)
}

type EtcdProvider interface {
	// CreateEtcdSession 创建一个新的 EtcdSession。
	// 调用者负责在不再需要时调用 Session 的 Close 方法。
	// 如果初始连接或租约授予失败，此方法会 panic。
	CreateEtcdSession(ctx context.Context) EtcdSession
}

type EtcdKvItem struct {
	Key         string
	Value       string
	ModRevision EtcdRevision
}

type EtcdRevision int64

// getEndpointsFromEnv 从环境变量获取 etcd 端点配置
// 返回：
// - 如果设置了 ETCD_ENDPOINTS 环境变量，返回解析后的端点列表
// - 否则返回默认值 ["localhost:2379"]
func getEndpointsFromEnv() []string {
	if endpoints := os.Getenv("ETCD_ENDPOINTS"); endpoints != "" {
		return strings.Split(endpoints, ",")
	}
	return []string{"localhost:2379"}
}

// getDialTimeoutFromEnv 从环境变量获取连接超时配置
// 返回：
// - 如果设置了 ETCD_DIAL_TIMEOUT 环境变量且为有效整数，返回该值
// - 否则返回默认值 5000（毫秒）
func getDialTimeoutMsFromEnv() int {
	return kcommon.GetEnvInt("ETCD_TIMEOUT_MS", 5000)
}
