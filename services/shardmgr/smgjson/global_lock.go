package smgjson

import (
	"encoding/json"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
)

type GlobalLock struct {
	PodName           string `json:"pod_name"`
	SessionId         string `json:"session_id"`
	LeaseId           int64  `json:"lease_id"`
	ServerStartTimeMs int64  `json:"server_start_time_ms"`
	Version           string `json:"version"`
	LastUpdateTimeMs  int64  `json:"last_update_time_ms"`
	LastUpdateResason string `json:"last_update_reason"`
}

func NewGlobalLock(podName string, sessionId string, leaseId int64, serverStartTimeMs int64, version string) *GlobalLock {
	return &GlobalLock{
		PodName:           podName,
		SessionId:         sessionId,
		LeaseId:           leaseId,
		ServerStartTimeMs: serverStartTimeMs,
		Version:           version,
	}
}

func (g *GlobalLock) ToJson() string {
	data, err := json.Marshal(g)
	if err != nil {
		ke := kerror.Wrap(err, "MarshalFailed", "GlobalLock", false)
		panic(ke)
	}
	return string(data)
}
