package smgjson

type WorkerConfigJson struct {
	// MaxAssignmentCountPerWorker 是 per worker 的最大 assignment 数量限制
	MaxAssignmentCountPerWorker *int32
	// Offline Grace Period sec 是 worker 下线的宽限期
	OfflineGracePeriodSec *int32
}
