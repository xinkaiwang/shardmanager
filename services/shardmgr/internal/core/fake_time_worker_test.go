package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

// TestWorkerGracePeriodExpiration 测试 worker 优雅期过期后的状态变化
// 这个测试使用 FakeTimeProvider 来精确控制时间流逝，避免真实等待
func TestWorkerGracePeriodExpiration(t *testing.T) {
	ctx := context.Background()
	klogging.SetDefaultLogger(klogging.NewLogrusLogger(ctx).SetConfig(ctx, "debug", "text"))

	// 配置测试环境
	setup := NewFakeTimeTestSetup(t)
	// 更新 ServiceConfig 中的 OfflineGracePeriodSec 为 10 秒
	gracePeriodSec := int32(10)
	setup.SetupBasicConfig(ctx, WithOfflineGracePeriodSec(gracePeriodSec))
	fakeTime := setup.FakeTime
	t.Logf("测试环境已配置")

	fn := func() {
		// 创建 ServiceState
		ss := NewServiceState(ctx, "TestWorkerGracePeriodExpiration")
		t.Logf("ServiceState已创建: %s", ss.Name)

		{
			// 验证 ServiceState 中的优雅期配置
			actrualGracePeriodSec := int32(0)
			safeAccessServiceState(ss, func(ss *ServiceState) {
				actrualGracePeriodSec = ss.ServiceConfig.WorkerConfig.OfflineGracePeriodSec
			})
			assert.Equal(t, gracePeriodSec, actrualGracePeriodSec, "ServiceState中的优雅期应与配置一致")
		}

		// 创建 worker-1 eph
		workerFullId, ephPath := ftCreateAndSetWorkerEph(t, ss, setup, "worker-1", "session-1", "localhost:8080")
		workerStatePath := ss.PathManager.FmtWorkerStatePath(workerFullId)

		{
			// worker 未创建时，状态应为 WS_Unknown
			workerStateEnum := safeGetWorkerStateEnum(ss, workerFullId)
			assert.Equal(t, data.WS_Unknown, workerStateEnum, "worker 未创建")
			// 等待worker state创建
			setup.FakeTime.VirtualTimeForward(ctx, 1000)
			workerStateEnum = safeGetWorkerStateEnum(ss, workerFullId)
			assert.Equal(t, data.WS_Online_healthy, workerStateEnum, "worker state 已创建")
		}

		// 确认worker初始状态
		workerState, _ := getWorkerStateAndPath(t, ss, workerFullId)
		t.Logf("worker初始状态验证完成: State=%v", workerState.State)
		assert.Equal(t, data.WS_Online_healthy, workerState.State, "初始状态应为健康在线")

		// 删除临时节点，触发临时节点丢失处理
		t.Logf("从etcd删除worker eph节点: %s", ephPath)
		setup.FakeEtcd.Delete(ctx, ephPath)
		t.Logf("已删除worker eph节点，等待状态同步")

		// 等待worker状态变为离线优雅期状态
		t.Logf("等待worker状态从online_healthy变为offline_graceful_period")

		// 推进0.5秒
		fakeTime.VirtualTimeForward(ctx, 500)

		{
			// 0.5 秒后应该还是 WS_Offline_graceful_period
			workerStateEnum := safeGetWorkerStateEnum(ss, workerFullId)
			assert.Equal(t, data.WS_Offline_graceful_period, workerStateEnum, "worker状态应变为 WS_Offline_graceful_period")
		}

		// 推进15秒，确保状态变化 (OfflineGracePeriodSec 为 10 秒)
		fakeTime.VirtualTimeForward(ctx, 15*1000)

		{
			// 15 秒后应该变为 WS_Offline_draining_complete
			workerStateEnum := safeGetWorkerStateEnum(ss, workerFullId)
			assert.Equal(t, data.WS_Offline_draining_complete, workerStateEnum, "worker状态应变为 WS_Offline_draining_complete")
			// 验证最终状态持久化
			nodeStr := setup.FakeStore.GetByKey(workerStatePath)
			ws := smgjson.WorkerStateJsonFromJson(nodeStr)
			assert.Equal(t, data.WS_Offline_draining_complete, ws.WorkerState, "worker state 状态已持久化")
		}

		// 推进25秒，确保状态变化 (DeleteGracePeriodSec 为 20 秒)
		fakeTime.VirtualTimeForward(ctx, 25*1000)

		{
			// 25 秒后应该变为 WS_Unknow (已删除)
			workerStateEnum := safeGetWorkerStateEnum(ss, workerFullId)
			assert.Equal(t, data.WS_Unknown, workerStateEnum, "worker状态应变为 WS_Unknown")
			// 验证最终状态持久化
			val := setup.FakeStore.GetByKey(workerStatePath)
			assert.Equal(t, "", val, "worker state 已删除")
		}
	}
	// 使用 FakeTimeProvider 和模拟的 EtcdProvider/EtcdStore 运行测试
	setup.RunWith(fn)
}

func TestWorkerGracePeriodExpiration_waitUntil(t *testing.T) {
	ctx := context.Background()
	klogging.SetDefaultLogger(klogging.NewLogrusLogger(ctx).SetConfig(ctx, "debug", "text"))

	// 配置测试环境
	setup := NewFakeTimeTestSetup(t)
	// 更新 ServiceConfig 中的 OfflineGracePeriodSec 为 10 秒
	gracePeriodSec := int32(10)
	setup.SetupBasicConfig(ctx, WithOfflineGracePeriodSec(gracePeriodSec))
	fakeTime := setup.FakeTime
	t.Logf("测试环境已配置")

	fn := func() {
		// 创建 ServiceState
		ss := NewServiceState(ctx, "TestWorkerGracePeriodExpiration_waitUntil")
		t.Logf("ServiceState已创建: %s", ss.Name)

		// 创建 worker-1 eph
		workerFullId, ephPath := ftCreateAndSetWorkerEph(t, ss, setup, "worker-1", "session-1", "localhost:8080")
		workerStatePath := ss.PathManager.FmtWorkerStatePath(workerFullId)

		{
			// worker 未创建时，状态应为 WS_Unknown
			workerStateEnum := safeGetWorkerStateEnum(ss, workerFullId)
			assert.Equal(t, data.WS_Unknown, workerStateEnum, "worker 未创建")

			// 等待worker state创建
			waitSucc, elapsedMs := WaitUntilWorkerState(t, ss, workerFullId, data.WS_Online_healthy, 1000, 10)
			assert.Equal(t, true, waitSucc, "worker state 已创建 elapsedMs=%d", elapsedMs)
		}

		// 删除临时节点，触发临时节点丢失处理
		t.Logf("从etcd删除worker eph节点: %s", ephPath)
		setup.FakeEtcd.Delete(ctx, ephPath)
		t.Logf("已删除worker eph节点，等待状态同步")

		{
			waitSucc, elapsedMs := WaitUntilWorkerState(t, ss, workerFullId, data.WS_Offline_graceful_period, 1000, 10)
			assert.Equal(t, true, waitSucc, "worker state 状态已变为 WS_Offline_graceful_period elapsedMs=%d", elapsedMs)
		}

		// 推进0.5秒
		fakeTime.VirtualTimeForward(ctx, 500)

		{
			// 0.5 秒后应该还是 WS_Offline_graceful_period
			workerStateEnum := safeGetWorkerStateEnum(ss, workerFullId)
			assert.Equal(t, data.WS_Offline_graceful_period, workerStateEnum, "worker状态应变为 WS_Offline_graceful_period")
		}

		{
			// 15 秒后应该变为 WS_Offline_draining_complete
			waitSucc, elapsedMs := WaitUntilWorkerState(t, ss, workerFullId, data.WS_Offline_draining_complete, 15*1000, 1000)
			assert.Equal(t, true, waitSucc, "worker state 状态已变为 WS_Offline_draining_complete elapsedMs=%d", elapsedMs)
		}

		{
			// 25 秒后应该变为 WS_Unknow (已删除)
			waitSucc, elapsedMs := WaitUntilWorkerState(t, ss, workerFullId, data.WS_Unknown, 30*1000, 1000)
			assert.Equal(t, true, waitSucc, "worker state 状态已变为 WS_Unknown elapsedMs=%d", elapsedMs)
		}
		{
			// 应该变为 WS_Unknow (已删除)
			workerStateEnum := safeGetWorkerStateEnum(ss, workerFullId)
			assert.Equal(t, data.WS_Unknown, workerStateEnum, "worker状态应变为 WS_Unknown")
			// 验证最终状态持久化
			val := setup.FakeStore.GetByKey(workerStatePath)
			assert.Equal(t, "", val, "worker state 已删除")
		}
	}
	// 使用 FakeTimeProvider 和模拟的 EtcdProvider/EtcdStore 运行测试
	setup.RunWith(fn)
}
