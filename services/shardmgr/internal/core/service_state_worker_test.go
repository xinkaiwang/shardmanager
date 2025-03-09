package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/services/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/etcdprov"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/shadow"
)

// TestServiceState_WorkerEphToState 测试ServiceState能否正确从worker eph创建worker state
func TestServiceState_WorkerEphToState(t *testing.T) {
	ctx := context.Background()

	// 重置全局状态
	resetGlobalState(t)
	t.Logf("全局状态已重置")

	// 配置测试环境
	setup := CreateTestSetup(t)
	setup.SetupBasicConfig(t)
	t.Logf("测试环境已配置")

	// 初始化ServiceState
	etcdprov.RunWithEtcdProvider(setup.FakeEtcd, func() {
		shadow.RunWithEtcdStore(setup.FakeStore, func() {
			setup.CreateServiceState(t, 0)
			ss := setup.ServiceState
			t.Logf("ServiceState已初始化")

			// 验证初始状态：无worker
			withServiceStateSync(ss, func(ss *ServiceState) {
				assert.Equal(t, 0, len(ss.AllWorkers), "初始状态应该没有worker")
				t.Logf("初始状态验证完成：AllWorkers数量=%d", len(ss.AllWorkers))
			})

			// 添加worker eph节点
			workerFullId := data.NewWorkerFullId("worker-1", "session-1", ss.IsStateInMemory())
			workerEph := cougarjson.NewWorkerEphJson("worker-1", "session-1", 1234567890000, 60)
			workerEph.AddressPort = "localhost:8080"
			workerEphJson, _ := json.Marshal(workerEph)
			t.Logf("已创建worker eph对象，worker-1:session-1")

			// 直接设置到etcd
			ephPath := ss.PathManager.GetWorkerEphPathPrefix() + workerFullId.String()
			t.Logf("设置worker eph节点到etcd路径: %s", ephPath)
			setup.FakeEtcd.Set(ctx, ephPath, string(workerEphJson))
			t.Logf("worker eph节点已设置到etcd，开始等待状态同步")

			// 等待系统自动处理worker eph变化并创建worker state
			success, elapsedMs := waitForWorkerStateCreation(t, ss, "worker-1")
			t.Logf("等待worker-1创建结果: success=%v, 耗时=%dms", success, elapsedMs)
			assert.True(t, success, "应该在超时前成功创建worker state: worker-1, elapsedMs=%d ms", elapsedMs)

			// 验证worker state已创建
			withServiceStateSync(ss, func(ss *ServiceState) {
				assert.Equal(t, 1, len(ss.AllWorkers), "应该有1个worker")
				worker, exists := ss.AllWorkers[workerFullId]
				t.Logf("验证worker-1，exists=%v", exists)
				assert.True(t, exists, "应该能找到worker-1")
				assert.Equal(t, data.WorkerId("worker-1"), worker.WorkerId, "worker ID应该正确")
				assert.Equal(t, data.SessionId("session-1"), worker.SessionId, "session ID应该正确")
				assert.Equal(t, data.WS_Online_healthy, worker.State, "worker应该处于健康状态")
				t.Logf("worker-1状态验证完成: ID=%v, Session=%v, State=%v",
					worker.WorkerId, worker.SessionId, worker.State)
			})

			// 添加第二个worker eph节点
			worker2FullId := data.NewWorkerFullId("worker-2", "session-2", ss.IsStateInMemory())
			worker2Eph := cougarjson.NewWorkerEphJson("worker-2", "session-2", 1234567890000, 60)
			worker2Eph.AddressPort = "localhost:8081"
			worker2EphJson, _ := json.Marshal(worker2Eph)
			t.Logf("已创建worker eph对象，worker-2:session-2")

			// 直接设置到etcd
			eph2Path := ss.PathManager.GetWorkerEphPathPrefix() + worker2FullId.String()
			t.Logf("设置worker eph节点到etcd路径: %s", eph2Path)
			setup.FakeEtcd.Set(ctx, eph2Path, string(worker2EphJson))
			t.Logf("worker eph节点已设置到etcd，开始等待状态同步")

			// 等待系统自动处理worker eph变化并创建worker state
			success, elapsedMs = waitForWorkerStateCreation(t, ss, "worker-2")
			t.Logf("等待worker-2创建结果: success=%v, 耗时=%dms", success, elapsedMs)
			assert.True(t, success, "应该在超时前成功创建worker state: worker-2")

			// 验证两个worker state都已创建
			withServiceStateSync(ss, func(ss *ServiceState) {
				assert.Equal(t, 2, len(ss.AllWorkers), "应该有2个worker")
				t.Logf("最终状态验证：AllWorkers数量=%d", len(ss.AllWorkers))

				// 验证worker-1
				worker1, exists1 := ss.AllWorkers[workerFullId]
				assert.True(t, exists1, "应该能找到worker-1")
				assert.Equal(t, data.WorkerId("worker-1"), worker1.WorkerId)
				assert.Equal(t, data.WS_Online_healthy, worker1.State)
				t.Logf("worker-1最终状态: exists=%v, State=%v", exists1, worker1.State)

				// 验证worker-2
				worker2, exists2 := ss.AllWorkers[worker2FullId]
				assert.True(t, exists2, "应该能找到worker-2")
				assert.Equal(t, data.WorkerId("worker-2"), worker2.WorkerId)
				assert.Equal(t, data.WS_Online_healthy, worker2.State)
				t.Logf("worker-2最终状态: exists=%v, State=%v", exists2, worker2.State)
			})
		})
	})
}

// waitForWorkerStateCreation 等待指定的worker state被创建
// 使用WaitUntil函数实现
func waitForWorkerStateCreation(t *testing.T, ss *ServiceState, workerId string) (bool, int64) {
	t.Helper()

	startMs := kcommon.GetMonoTimeMs()
	fn := func() (bool, string) {
		// 不再创建固定的 workerFullId，而是遍历 AllWorkers 查找
		var workerIds []string
		exists := false

		// 记录所有 worker IDs 用于调试
		for id := range ss.AllWorkers {
			workerIds = append(workerIds, id.String())

			// 检查是否有以指定 workerId 开头的 workerFullId
			if strings.HasPrefix(id.String(), workerId+":") {
				exists = true
			}
		}

		// 生成调试信息字符串
		debugInfo := fmt.Sprintf("查找workerId=%s, workers=%v, exists=%v",
			workerId, workerIds, exists)

		t.Logf("检查worker state: %s", debugInfo)

		return exists, debugInfo
	}
	// 使用已有的WaitUntil函数实现等待
	succ := WaitUntil(t, fn, 5000, 100)
	return succ, kcommon.GetMonoTimeMs() - startMs
}

// // waitForWorkerStateWithTimeout 等待worker state被创建，指定超时和间隔时间
// func waitForWorkerStateWithTimeout(t *testing.T, ss *ServiceState, workerId string, maxWaitMs int, intervalMs int) bool {
// 	t.Helper()

// 	startTime := time.Now()
// 	maxDuration := time.Duration(maxWaitMs) * time.Millisecond
// 	intervalDuration := time.Duration(intervalMs) * time.Millisecond

// 	for i := 0; i < maxWaitMs/intervalMs; i++ {
// 		exists := false
// 		var workerIds []string

// 		withServiceStateSync(ss, func(ss *ServiceState) {
// 			// 收集当前所有worker ID用于调试输出
// 			for _, worker := range ss.AllWorkers {
// 				workerIds = append(workerIds, string(worker.WorkerId))

// 				// 检查指定worker是否存在
// 				if string(worker.WorkerId) == workerId {
// 					exists = true
// 				}
// 			}
// 		})

// 		if exists {
// 			elapsed := time.Since(startTime)
// 			t.Logf("找到worker %s，耗时 %v", workerId, elapsed)
// 			return true
// 		}

// 		elapsed := time.Since(startTime)
// 		if elapsed >= maxDuration {
// 			t.Logf("等待worker %s 创建超时，已尝试 %d 次，耗时 %v，当前worker: %v",
// 				workerId, i+1, elapsed, workerIds)
// 			return false
// 		}

// 		t.Logf("等待worker %s 创建中 (尝试 %d/%d)，已耗时 %v，当前worker: %v",
// 			workerId, i+1, maxWaitMs/intervalMs, elapsed, workerIds)
// 		time.Sleep(intervalDuration)
// 	}

// 	return false
// }
