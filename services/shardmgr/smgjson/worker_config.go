package smgjson

type WorkerConfigJson struct {
	// MaxAssignmentCountPerWorker 是 per worker 的最大 assignment 数量限制
	MaxAssignmentCountPerWorker *int32 `json:"max_assignment_count_per_worker,omitempty"`
	// Offline Grace Period sec 是 worker 下线的宽限期
	OfflineGracePeriodSec *int32 `json:"offline_grace_period_sec,omitempty"`
}
