package smgjson

import (
	"github.com/xinkaiwang/shardmanager/services/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

type WorkerInfoJson struct {
	WorkerId     data.WorkerId     `json:"worker_id"`
	SessionId    data.SessionId    `json:"session_id"`
	AddressPort  string            `json:"address_port"`
	StartTimeMs  int64             `json:"start_time_ms"`
	Capacity     int32             `json:"capacity"`
	MemorySizeMB int32             `json:"memory_size_mb"`
	Properties   map[string]string `json:"properties,omitempty"`
}

func WorkerInfoFromWorkerEph(workerEph *cougarjson.WorkerEphJson) WorkerInfoJson {
	return WorkerInfoJson{
		WorkerId:     data.WorkerId(workerEph.WorkerId),
		SessionId:    data.SessionId(workerEph.SessionId),
		AddressPort:  workerEph.AddressPort,
		StartTimeMs:  workerEph.StartTimeMs,
		Capacity:     workerEph.Capacity,
		MemorySizeMB: workerEph.MemorySizeMB,
		Properties:   workerEph.Properties,
	}
}

func (wi *WorkerInfoJson) Equals(other *WorkerInfoJson) bool {
	if wi.WorkerId != other.WorkerId {
		return false
	}
	if wi.SessionId != other.SessionId {
		return false
	}
	if wi.AddressPort != other.AddressPort {
		return false
	}
	if wi.StartTimeMs != other.StartTimeMs {
		return false
	}
	if wi.Capacity != other.Capacity {
		return false
	}
	if wi.MemorySizeMB != other.MemorySizeMB {
		return false
	}
	if !StringMapEquals(wi.Properties, other.Properties) {
		return false
	}
	return true
}

func StringMapEquals(map1 map[string]string, map2 map[string]string) bool {
	if len(map1) != len(map2) {
		return false
	}
	for key, value := range map1 {
		if map2[key] != value {
			return false
		}
	}
	return true
}
