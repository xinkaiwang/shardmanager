package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

// TestWorkerGracePeriodExpiration 测试 worker 优雅期过期后的状态变化
// 1. 工作节点处于正常"在线健康"状态
// 2. 节点突然离线（临时节点被删除）
// 3. 推进0.5秒，节点状态转变为"离线优雅期"（WS_Offline_graceful_period）
// 4. 推进15秒（超过10秒优雅期），节点状态转为"排空完成"（WS_Offline_draining_complete）
// 5. 推进25秒（超过20秒删除优雅期），节点状态变为"未知"（WS_Unknown），且被完全删除

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

// 这个测试与前一个测试验证相同的功能，但使用了不同的测试方法。它使用WaitUntilWorkerState函数主动等待特定状态的出现，而不是仅依赖于时间推进后的状态检查。这种方法更接近实际系统的行为模式，可以测试状态转换的时间特性和系统反应。
// 主要区别是：
// 1. TestWorkerGracePeriodExpiration使用直接的时间推进和状态检查
// 2. TestWorkerGracePeriodExpiration_waitUntil使用WaitUntilWorkerState函数来等待状态变化

// 1. 工作节点处于"未知"状态（WS_Unknown）
// 2. 使用WaitUntilWorkerState等待节点状态变为"在线健康"（WS_Online_healthy）
// 3. 节点突然离线（临时节点被删除）
// 4. 使用WaitUntilWorkerState等待节点状态变为"离线优雅期"（WS_Offline_graceful_period）
// 5. 推进0.5秒并验证状态仍为"离线优雅期"
// 6. 使用WaitUntilWorkerState等待15秒直到节点状态变为"排空完成"（WS_Offline_draining_complete）
// 7. 使用WaitUntilWorkerState等待30秒直到节点状态变为"未知"（WS_Unknown）并被删除

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

// 这个测试验证了ShardManager的有序关闭机制：工作节点可以通过更新其临时节点数据来请求关闭，系统能够检测并适当地更新节点状态。这种机制允许工作节点在退出前进行优雅关闭，完成当前任务并避免新任务分配，实现平滑的服务降级。

// 1. 创建并初始化ServiceState
// 2. 在etcd中创建并设置一个工作节点临时数据（worker-1:session-1）
// 3. 验证工作节点初始状态为"在线健康"（WS_Online_healthy）且ShutdownRequesting为false
// 4. 更新etcd中的工作节点临时数据，将ReqShutDown设置为1（请求关闭）
// 5. 等待工作节点状态变为"在线关闭请求"（WS_Online_shutdown_req）
// 6. 验证最终状态正确：
//    - 状态已变为WS_Online_shutdown_req
//    - ShutdownRequesting标志已设置为true
// 7. 确认工作节点状态已正确持久化到存储中

// TestWorkerShutdownRequest 测试通过worker eph节点请求关闭的流程
func TestWorkerShutdownRequest(t *testing.T) {
	ctx := context.Background()
	klogging.SetDefaultLogger(klogging.NewLogrusLogger(ctx).SetConfig(ctx, "debug", "text"))

	// 配置测试环境
	setup := NewFakeTimeTestSetup(t)
	setup.SetupBasicConfig(ctx)
	t.Logf("测试环境已配置")

	fn := func() {
		// 创建 ServiceState
		ss := NewServiceState(ctx, "TestWorkerShutdownRequest")
		t.Logf("ServiceState已创建: %s", ss.Name)

		// 创建 worker-1 eph
		workerFullId, ephPath := ftCreateAndSetWorkerEph(t, ss, setup, "worker-1", "session-1", "localhost:8080")
		workerStatePath := ss.PathManager.FmtWorkerStatePath(workerFullId)

		{
			// 等待worker state创建，状态变为WS_Online_healthy
			waitSucc, elapsedMs := WaitUntilWorkerState(t, ss, workerFullId, data.WS_Online_healthy, 1000, 10)
			assert.Equal(t, true, waitSucc, "worker state 已创建 elapsedMs=%d", elapsedMs)
		}

		{
			// 验证初始ShutdownRequesting状态为false
			var shutdownRequesting bool
			safeAccessWorkerById(ss, workerFullId, func(ws *WorkerState) {
				shutdownRequesting = ws.ShutdownRequesting
			})
			assert.Equal(t, false, shutdownRequesting, "初始ShutdownRequesting应为false")
		}

		// 更新worker eph节点，设置ReqShutDown=1
		t.Logf("更新worker eph节点，设置ReqShutDown=1")
		ftUpdateWorkerEphWithShutdownRequest(t, ss, setup, ephPath)
		t.Logf("已更新worker eph节点，等待状态同步")

		// 等待worker状态变为WS_Online_shutdown_req
		t.Logf("等待worker状态从WS_Online_healthy变为WS_Online_shutdown_req")
		{
			waitSucc, elapsedMs := WaitUntilWorkerState(t, ss, workerFullId, data.WS_Online_shutdown_req, 1000, 10)
			assert.Equal(t, true, waitSucc, "worker state 状态已变为 WS_Online_shutdown_req elapsedMs=%d", elapsedMs)
		}

		{
			// 验证ShutdownRequesting状态为true
			var shutdownRequesting bool
			safeAccessWorkerById(ss, workerFullId, func(ws *WorkerState) {
				shutdownRequesting = ws.ShutdownRequesting
			})
			assert.Equal(t, true, shutdownRequesting, "ShutdownRequesting应为true")
		}

		// 验证worker状态持久化
		t.Logf("验证worker状态持久化")
		{
			nodeStr := setup.FakeStore.GetByKey(workerStatePath)
			ws := smgjson.WorkerStateJsonFromJson(nodeStr)
			assert.Equal(t, data.WS_Online_shutdown_req, ws.WorkerState, "worker state 状态已持久化")
		}
	}
	// 使用 FakeTimeProvider 和模拟的 EtcdProvider/EtcdStore 运行测试
	setup.RunWith(fn)
}

// ftUpdateWorkerEphWithShutdownRequest 更新worker eph，设置关闭请求标志
func ftUpdateWorkerEphWithShutdownRequest(t *testing.T, ss *ServiceState, setup *FakeTimeTestSetup, ephPath string) {
	// 获取当前eph内容
	kvItem := setup.FakeEtcd.Get(setup.ctx, ephPath)
	assert.NotEqual(t, "", kvItem.Value, "应能找到worker eph节点")

	// 解析当前eph内容
	workerEph := cougarjson.WorkerEphJsonFromJson(kvItem.Value)
	// 设置关闭请求
	workerEph.ReqShutDown = 1 // 设置为true
	// 更新到etcd
	setup.FakeEtcd.Set(setup.ctx, ephPath, workerEph.ToJson())
}
