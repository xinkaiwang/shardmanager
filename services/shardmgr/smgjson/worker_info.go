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
