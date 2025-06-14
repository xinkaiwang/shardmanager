package smgjson

import (
	"encoding/json"

	"github.com/xinkaiwang/shardmanager/libs/cougar/cougarjson"
)

type WorkerInfoJson struct {
	AddressPort  string            `json:"address_port"`
	StartTimeMs  int64             `json:"start_time_ms"`
	Capacity     int32             `json:"capacity"`
	MemorySizeMB int32             `json:"memory_size_mb"`
	Properties   map[string]string `json:"properties,omitempty"`
}

func NewWorkerInfoJson() *WorkerInfoJson {
	return &WorkerInfoJson{}
}

func WorkerInfoFromWorkerEph(workerEph *cougarjson.WorkerEphJson) *WorkerInfoJson {
	return &WorkerInfoJson{
		AddressPort:  workerEph.AddressPort,
		StartTimeMs:  workerEph.StartTimeMs,
		Capacity:     workerEph.Capacity,
		MemorySizeMB: workerEph.MemorySizeMB,
		Properties:   workerEph.Properties,
	}
}

func (wi *WorkerInfoJson) Equals(other *WorkerInfoJson) bool {
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

func (wi *WorkerInfoJson) ToJson() string {
	bytes, err := json.Marshal(wi)
	if err != nil {
		panic(err)
	}
	return string(bytes)
}
