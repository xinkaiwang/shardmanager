package cougarjson

type ShardStats struct {
	Qpm   int64 `json:"qpm"`              // query per minute
	MemMb int64 `json:"mem_mb,omitempty"` // memory usage
}

type PodStats struct {
	MemMb int64 `json:"mem_mb,omitempty"` // memory usage
	CpuMs int64 `json:"cpu_ms,omitempty"` // cpu usage
}
