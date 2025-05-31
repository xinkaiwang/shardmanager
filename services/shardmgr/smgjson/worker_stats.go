package smgjson

import "encoding/json"

type WorkerStatsJson struct {
	CpuUsageMs int64 `json:"cpu_ms"`
	MemUsageMB int64 `json:"mem_mb"`
}

func NewWorkerStatsJson(cpuUsageMs, memUsageMB int64) *WorkerStatsJson {
	return &WorkerStatsJson{
		CpuUsageMs: cpuUsageMs,
		MemUsageMB: memUsageMB,
	}
}
func (ws *WorkerStatsJson) ToJson() string {
	bytes, err := json.Marshal(ws)
	if err != nil {
		panic(err) // Handle error appropriately in production code
	}
	return string(bytes)
}
func WorkerStatsJsonFromJson(jsonStr string) *WorkerStatsJson {
	var ws WorkerStatsJson
	err := json.Unmarshal([]byte(jsonStr), &ws)
	if err != nil {
		panic(err) // Handle error appropriately in production code
	}
	return &ws
}
func (ws *WorkerStatsJson) Equals(other *WorkerStatsJson) bool {
	if ws == nil && other == nil {
		return true
	}
	if ws == nil || other == nil {
		return false
	}
	if ws.CpuUsageMs != other.CpuUsageMs {
		return false
	}
	if ws.MemUsageMB != other.MemUsageMB {
		return false
	}
	return true
}
