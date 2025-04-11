package etcd

import (
	"context"
	"os"
	"strings"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
)

var (
	etcdTimeoutMs = kcommon.GetEnvInt("ETCD_TIMEOUT_MS", 5000)
)

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
	PutNode(key string, value string)
	DeleteNode(key string)
	SetStateListener(listener EtcdStateListener)
	GetCurrentState() EtcdSessionState
}

type EtcdProvider interface {
	CreateEtcdSession() EtcdSession
	// WatchPrefix
	WatchByPrefix(ctx context.Context, pathPrefix string, revision EtcdRevision) chan EtcdKvItem
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
// - 否则返回默认值 5（秒）
func getDialTimeoutMsFromEnv() int {
	return kcommon.GetEnvInt("ETCD_TIMEOUT_MS", 5000)
}
