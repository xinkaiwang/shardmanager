package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

// 辅助函数

// getWorkerStateAndPath 获取worker state和路径
func getWorkerStateAndPath(t *testing.T, ss *ServiceState, workerFullId data.WorkerFullId) (*WorkerState, string) {
	var workerState *WorkerState
	var workerStatePath string

	safeAccessServiceState(ss, func(ss *ServiceState) {
		worker, exists := ss.AllWorkers[workerFullId]
		assert.True(t, exists, "worker应该已创建")
		workerState = worker
		workerStatePath = ss.PathManager.FmtWorkerStatePath(workerFullId)
	})

	return workerState, workerStatePath
}

// WithOfflineGracePeriodSec 设置 WorkerConfig 的离线优雅期 (default=10s)
func WithOfflineGracePeriodSec(seconds int32) smgjson.ServiceConfigOption {
	return func(cfg *smgjson.ServiceConfigJson) {
		cfg.WorkerConfig.OfflineGracePeriodSec = smgjson.NewInt32Pointer(seconds)
	}
}
