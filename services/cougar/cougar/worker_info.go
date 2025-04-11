package cougar

import "github.com/xinkaiwang/shardmanager/services/unicorn/data"

type WorkerInfo struct {
	// WorkerId 是工作节点的唯一标识符, 通常是 hostname，也就是 pod name
	WorkerId data.WorkerId

	// SessionId unique ID to identity current process, changes in each restart/reboot.
	SessionId data.SessionId

	AddressPort string // for exp: "10.0.0.32:8080"

	// StartTimeMs 是工作节点启动的时间戳（毫秒）
	StartTimeMs int64

	// Capacity 表示工作节点的处理容量 (100 means 100%)
	Capacity int32

	MemorySizeMB int32 // how many memory (or vmem, depend on which resource is on the critical path) in this worker

	// Properties 存储工作节点的额外属性
	// 键和值都是字符串类型
	Properties map[string]string // gpu_ct="1", gpu_type="H100", etc.
}
