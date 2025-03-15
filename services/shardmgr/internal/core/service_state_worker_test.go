package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/etcdprov"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/shadow"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

// TestServiceState_WorkerEphToState 测试ServiceState能否正确从worker eph创建worker state
func TestServiceState_WorkerEphToState(t *testing.T) {
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
			safeAccessServiceState(ss, func(ss *ServiceState) {
				assert.Equal(t, 0, len(ss.AllWorkers), "初始状态应该没有worker")
				t.Logf("初始状态验证完成：AllWorkers数量=%d", len(ss.AllWorkers))
			})

			// 创建并设置第一个worker eph
			workerFullId1, _ := createAndSetWorkerEph(t, ss, setup, "worker-1", "session-1", "localhost:8080")

			// 验证第一个worker state已创建并且状态正确
			validateWorkerState(t, ss, "worker-1", "session-1", data.WS_Online_healthy)

			// 创建并设置第二个worker eph
			workerFullId2, _ := createAndSetWorkerEph(t, ss, setup, "worker-2", "session-2", "localhost:8081")

			// 验证第二个worker state已创建并且状态正确
			validateWorkerState(t, ss, "worker-2", "session-2", data.WS_Online_healthy)

			// 验证总worker数量
			safeAccessServiceState(ss, func(ss *ServiceState) {
				assert.Equal(t, 2, len(ss.AllWorkers), "应该有2个worker")
				t.Logf("最终状态验证：AllWorkers数量=%d", len(ss.AllWorkers))
			})

			// 等待worker state被持久化到存储并验证
			worker1StatePath := ss.PathManager.FmtWorkerStatePath(workerFullId1)
			worker2StatePath := ss.PathManager.FmtWorkerStatePath(workerFullId2)

			// 等待并验证worker-1状态持久化
			validateWorkerStatePersistence(t, setup.FakeStore, worker1StatePath)

			// 等待并验证worker-2状态持久化
			validateWorkerStatePersistence(t, setup.FakeStore, worker2StatePath)

			// 验证FakeEtcdStore内容
			validateFakeEtcdStoreContents(t, ss, setup.FakeStore)
		})
	})
}

// TestWorkerState_EphNodeLost 测试临时节点丢失时的状态转换
func TestWorkerState_EphNodeLost(t *testing.T) {
	t.Run("worker ephemeral node lost", func(t *testing.T) {
		ctx := context.Background()

		// 减少日志输出，使测试更清晰
		setupNullLogger(t)

		// 重置全局状态
		resetGlobalState(t)
		t.Logf("全局状态已重置")

		// 配置测试环境
		setup := CreateTestSetup(t)
		setup.SetupBasicConfig(t)
		t.Logf("测试环境已配置")

		etcdprov.RunWithEtcdProvider(setup.FakeEtcd, func() {
			shadow.RunWithEtcdStore(setup.FakeStore, func() {
				setup.CreateServiceState(t, 0)
				ss := setup.ServiceState
				t.Logf("ServiceState已初始化")

				// 创建并设置worker eph
				workerFullId, ephPath := createAndSetWorkerEph(t, ss, setup, "worker-1", "session-1", "localhost:8080")

				// 确认worker初始状态
				workerState, workerStatePath := getWorkerStateAndPath(t, ss, workerFullId)
				t.Logf("worker初始状态验证完成: State=%v", workerState.State)
				initialState := workerState.State
				t.Logf("worker初始状态: %v", initialState)

				// 删除临时节点，触发临时节点丢失处理
				t.Logf("从etcd删除worker eph节点: %s", ephPath)
				setup.FakeEtcd.Delete(ctx, ephPath)
				t.Logf("已删除worker eph节点，等待状态同步")

				// 等待worker状态变为离线状态
				t.Logf("等待worker状态从online_healthy变为offline_graceful_period")
				// waitForWorkerStateChange(t, ss, workerFullId, data.WS_Offline_graceful_period)
				waitSucc, elapsedMs := WaitUntilWorkerState(t, ss, workerFullId, data.WS_Offline_graceful_period, 1000, 50)
				assert.True(t, waitSucc, "应该在超时前成功变为 offline_graceful_period elapsedMs=%d", elapsedMs)

				// 验证最终状态
				finalWorkerState, _ := getWorkerStateAndPath(t, ss, workerFullId)
				assert.Equal(t, data.WS_Offline_graceful_period, finalWorkerState.State,
					"worker状态应变为WS_Offline_graceful_period")
				assert.True(t, finalWorkerState.GracePeriodStartTimeMs > 0,
					"优雅期开始时间应被设置")
				t.Logf("worker状态已验证: 原状态=%v, 新状态=%v, GracePeriodStartTimeMs=%d",
					initialState, finalWorkerState.State, finalWorkerState.GracePeriodStartTimeMs)

				// 验证worker状态持久化
				validateWorkerStatePersistenceWithState(t, setup.FakeStore, workerStatePath,
					"worker-1", data.WS_Offline_graceful_period)
			})
		})
	})
}

// TestWorkerShutdownRequest 测试通过worker eph节点请求关闭的流程
func TestWorkerShutdownRequest(t *testing.T) {
	ctx := context.Background()

	// 减少日志输出，使测试更清晰
	setupNullLogger(t)

	// 重置全局状态
	resetGlobalState(t)
	t.Logf("全局状态已重置")

	// 配置测试环境
	setup := CreateTestSetup(t)
	setup.SetupBasicConfig(t)
	t.Logf("测试环境已配置")

	etcdprov.RunWithEtcdProvider(setup.FakeEtcd, func() {
		shadow.RunWithEtcdStore(setup.FakeStore, func() {
			setup.CreateServiceState(t, 0)
			ss := setup.ServiceState
			t.Logf("ServiceState已初始化")

			// 创建并设置worker eph
			workerFullId, ephPath := createAndSetWorkerEph(t, ss, setup, "worker-1", "session-1", "localhost:8080")

			// It's not mutable so let's reuse it
			_, workerStatePath := getWorkerStateAndPath(t, ss, workerFullId)

			// 验证初始状态
			validateInitialWorkerState(t, ss, workerFullId)

			// 更新worker eph，设置关闭请求标志
			t.Logf("更新worker eph，设置ReqShutDown=1")
			updateWorkerEphWithShutdownRequest(t, setup, ctx, ephPath)
			t.Logf("已更新worker eph节点，等待状态同步")

			// 等待worker状态变为shutdown_req
			t.Logf("等待worker状态变为online_shutdown_req")
			waitForWorkerShutdownRequest(t, ss, workerFullId)
			t.Logf("worker状态已成功变为shutdown_req")

			// 验证最终状态
			validateWorkerFinalState(t, ss, workerFullId, data.WS_Online_healthy, data.WS_Online_shutdown_req)

			// 验证状态持久化
			validateWorkerStatePersistenceWithState(t, setup.FakeStore, workerStatePath,
				"worker-1", data.WS_Online_shutdown_req)
		})
	})
}

// 辅助函数

// setupNullLogger 设置空日志记录器以减少输出
func setupNullLogger(t *testing.T) {
	originalLogger := klogging.GetDefaultLogger()
	nullLogger := klogging.NewLogrusLogger(context.Background())
	klogging.SetDefaultLogger(nullLogger)
	t.Cleanup(func() {
		klogging.SetDefaultLogger(originalLogger)
	})
}

// createAndSetWorkerEph 创建worker eph节点并设置到etcd
// 返回workerFullId和ephPath
func createAndSetWorkerEph(t *testing.T, ss *ServiceState, setup *ServiceStateTestSetup, workerId string, sessionId string, addrPort string) (data.WorkerFullId, string) {
	ctx := context.Background()

	// 创建workerId对象
	workerFullId := data.NewWorkerFullId(data.WorkerId(workerId), data.SessionId(sessionId), ss.IsStateInMemory())

	// 创建worker eph对象
	workerEph := cougarjson.NewWorkerEphJson(workerId, sessionId, 1234567890000, 60)
	workerEph.AddressPort = addrPort
	workerEphJson, _ := json.Marshal(workerEph)
	t.Logf("已创建worker eph对象，%s:%s", workerId, sessionId)

	// 设置到etcd
	ephPath := ss.PathManager.GetWorkerEphPathPrefix() + workerFullId.String()
	t.Logf("设置worker eph节点到etcd路径: %s", ephPath)
	setup.FakeEtcd.Set(ctx, ephPath, string(workerEphJson))

	// 等待worker state创建
	success, elapsedMs := waitForWorkerStateCreation(t, ss, workerId)
	t.Logf("等待%s创建结果: success=%v, 耗时=%dms", workerId, success, elapsedMs)
	assert.True(t, success, "应该在超时前成功创建worker state: %s", workerId)

	return workerFullId, ephPath
}

// validateWorkerState 验证worker state状态
func validateWorkerState(t *testing.T, ss *ServiceState, workerId string, sessionId string, expectedState data.WorkerStateEnum) {
	workerFullId := data.NewWorkerFullId(data.WorkerId(workerId), data.SessionId(sessionId), ss.IsStateInMemory())

	safeAccessServiceState(ss, func(ss *ServiceState) {
		worker, exists := ss.AllWorkers[workerFullId]
		t.Logf("验证%s，exists=%v", workerId, exists)
		assert.True(t, exists, "应该能找到%s", workerId)
		assert.Equal(t, data.WorkerId(workerId), worker.WorkerId, "worker ID应该正确")
		assert.Equal(t, data.SessionId(sessionId), worker.SessionId, "session ID应该正确")
		assert.Equal(t, expectedState, worker.State, "worker应该处于预期状态")
		t.Logf("%s状态验证完成: ID=%v, Session=%v, State=%v",
			workerId, worker.WorkerId, worker.SessionId, worker.State)
	})
}

// checkWorkerStatePersistence 验证worker state持久化
func checkWorkerStatePersistence(t *testing.T, store *shadow.FakeEtcdStore, workerStatePath string) {
	// 等待worker状态持久化
	t.Logf("等待worker state持久化到路径: %s", workerStatePath)
	success := store.GetByKey(workerStatePath)
	assert.True(t, success != "", "worker状态应成功持久化")
	t.Logf("等待worker状态持久化结果: success=%v", success)
}

// validateWorkerStatePersistence 验证worker state持久化
func validateWorkerStatePersistence(t *testing.T, store *shadow.FakeEtcdStore, workerStatePath string) {
	// 等待worker状态持久化
	t.Logf("等待worker state持久化到路径: %s", workerStatePath)
	success, waitDuration := waitForWorkerStatePersistence(t, store, workerStatePath)
	assert.True(t, success, "worker状态应成功持久化")
	t.Logf("等待worker状态持久化结果: success=%v, 等待时间=%dms", success, waitDuration)
}

// validateWorkerStatePersistenceWithState 验证持久化的worker state内容
func checkWorkerStatePersistenceWithState(t *testing.T, store *shadow.FakeEtcdStore, workerStatePath string,
	expectedWorkerId string, expectedState data.WorkerStateEnum) {

	// 验证状态持久化
	checkWorkerStatePersistence(t, store, workerStatePath)

	// 验证持久化的内容
	jsonStr := store.GetByKey(workerStatePath)
	storedWorkerState := smgjson.WorkerStateJsonFromJson(jsonStr)

	assert.NotNil(t, storedWorkerState, "应该能解析worker state json")
	assert.Equal(t, data.WorkerId(expectedWorkerId), storedWorkerState.WorkerId, "持久化的WorkerId应正确")
	assert.Equal(t, expectedState, storedWorkerState.WorkerState, "持久化的状态应符合预期")
	t.Logf("持久化数据验证完成: WorkerId=%s, State=%v",
		storedWorkerState.WorkerId, storedWorkerState.WorkerState)
}

// validateWorkerStatePersistenceWithState 验证持久化的worker state内容
func validateWorkerStatePersistenceWithState(t *testing.T, store *shadow.FakeEtcdStore, workerStatePath string,
	expectedWorkerId string, expectedState data.WorkerStateEnum) {

	// 验证状态持久化
	validateWorkerStatePersistence(t, store, workerStatePath)

	// 验证持久化的内容
	storeData := store.GetData()
	jsonStr := storeData[workerStatePath]
	storedWorkerState := smgjson.WorkerStateJsonFromJson(jsonStr)

	assert.NotNil(t, storedWorkerState, "应该能解析worker state json")
	assert.Equal(t, data.WorkerId(expectedWorkerId), storedWorkerState.WorkerId, "持久化的WorkerId应正确")
	assert.Equal(t, expectedState, storedWorkerState.WorkerState, "持久化的状态应符合预期")
	t.Logf("持久化数据验证完成: WorkerId=%s, State=%v",
		storedWorkerState.WorkerId, storedWorkerState.WorkerState)
}

// validateFakeEtcdStoreContents 验证FakeEtcdStore内容
func validateFakeEtcdStoreContents(t *testing.T, ss *ServiceState, store *shadow.FakeEtcdStore) {
	t.Log("验证FakeEtcdStore内容")
	storeData := store.GetData()
	t.Logf("FakeEtcdStore数据条数: %d", len(storeData))

	// 验证worker state是否被持久化
	workerStatePersisted := false

	for k, v := range storeData {
		if strings.HasPrefix(k, ss.PathManager.GetWorkerStatePathPrefix()) {
			workerStatePersisted = true
			t.Logf("  - worker state持久化: 键=%s, 值长度=%d", k, len(v))

			// 验证worker state内容
			var workerStateJson smgjson.WorkerStateJson
			err := json.Unmarshal([]byte(v), &workerStateJson)
			assert.NoError(t, err, "应该能解析worker state json")

			if err == nil {
				t.Logf("    worker state内容: WorkerId=%s, SessionId=%s, 状态=%v",
					workerStateJson.WorkerId, workerStateJson.SessionId, workerStateJson.WorkerState)
			}
		}
	}

	assert.True(t, workerStatePersisted, "worker state应该被持久化到FakeEtcdStore")
}

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

// // waitForWorkerStateChange 等待worker状态变为指定状态
// func waitForWorkerStateChange(t *testing.T, ss *ServiceState, workerFullId data.WorkerFullId, expectedState data.WorkerStateEnum) {
// 	waitForStateCondition := func() (bool, string) {
// 		var state data.WorkerStateEnum
// 		var exists bool

// 		safeAccessServiceState(ss, func(ss *ServiceState) {
// 			worker, ok := ss.AllWorkers[workerFullId]
// 			if !ok {
// 				exists = false
// 				return
// 			}
// 			exists = true
// 			state = worker.State
// 		})

// 		if !exists {
// 			return false, "worker不存在"
// 		}

// 		// 检查状态是否变为期望状态
// 		matched := state == expectedState
// 		return matched, fmt.Sprintf("worker状态=%v, 是否匹配=%v", state, matched)
// 	}

// 	if !WaitUntil(t, waitForStateCondition, 1000, 50) {
// 		t.Fatalf("等待超时，状态仍未变为 %v，测试失败", expectedState)
// 	}
// }

// updateWorkerEphWithShutdownRequest 更新worker eph，设置关闭请求标志
func updateWorkerEphWithShutdownRequest(t *testing.T, setup *ServiceStateTestSetup, ctx context.Context, ephPath string) {
	// 获取当前eph内容
	ephItems := setup.FakeEtcd.List(ctx, ephPath, 1)
	assert.Equal(t, 1, len(ephItems), "应能找到worker eph节点")

	currentEphJson := ephItems[0].Value
	updatedWorkerEph := cougarjson.WorkerEphJsonFromJson(currentEphJson)
	updatedWorkerEph.ReqShutDown = 1 // 设置为true
	updatedWorkerEphJson, _ := json.Marshal(updatedWorkerEph)

	// 更新etcd中的worker eph节点
	setup.FakeEtcd.Set(ctx, ephPath, string(updatedWorkerEphJson))
}

// validateInitialWorkerState 验证worker初始状态
func validateInitialWorkerState(t *testing.T, ss *ServiceState, workerFullId data.WorkerFullId) {
	var initialState data.WorkerStateEnum
	var initialShutdownRequesting bool

	safeAccessServiceState(ss, func(ss *ServiceState) {
		worker, exists := ss.AllWorkers[workerFullId]
		assert.True(t, exists, "worker应该已创建")
		initialState = worker.State
		initialShutdownRequesting = worker.ShutdownRequesting
		assert.Equal(t, data.WS_Online_healthy, worker.State, "worker初始状态应为healthy")
		assert.False(t, worker.ShutdownRequesting, "初始ShutdownRequesting应为false")
	})
	t.Logf("初始状态验证完成: 状态=%v, ShutdownRequesting=%v", initialState, initialShutdownRequesting)
}

// waitForWorkerShutdownRequest 等待worker状态变为shutdown_req
func waitForWorkerShutdownRequest(t *testing.T, ss *ServiceState, workerFullId data.WorkerFullId) {
	waitForShutdownReq := func() (bool, string) {
		var state data.WorkerStateEnum
		var shuttingDown bool
		var exists bool

		safeAccessServiceState(ss, func(ss *ServiceState) {
			worker, ok := ss.AllWorkers[workerFullId]
			if !ok {
				exists = false
				return
			}
			exists = true
			state = worker.State
			shuttingDown = worker.ShutdownRequesting
		})

		if !exists {
			return false, "worker不存在"
		}

		isShutdownReq := state == data.WS_Online_shutdown_req && shuttingDown
		return isShutdownReq, fmt.Sprintf("状态=%v, ShutdownRequesting=%v", state, shuttingDown)
	}

	if !WaitUntil(t, waitForShutdownReq, 1000, 50) {
		t.Fatalf("等待超时，worker状态未变为shutdown_req")
	}
}

// validateWorkerFinalState 验证worker最终状态
func validateWorkerFinalState(t *testing.T, ss *ServiceState, workerFullId data.WorkerFullId,
	initialState data.WorkerStateEnum, expectedState data.WorkerStateEnum) {

	var finalState data.WorkerStateEnum
	var finalShutdownRequesting bool

	safeAccessServiceState(ss, func(ss *ServiceState) {
		worker, exists := ss.AllWorkers[workerFullId]
		assert.True(t, exists, "worker应该存在")
		finalState = worker.State
		finalShutdownRequesting = worker.ShutdownRequesting
	})

	assert.Equal(t, expectedState, finalState, "worker状态应变为%v", expectedState)
	assert.True(t, finalShutdownRequesting, "ShutdownRequesting应为true")
	t.Logf("最终状态验证完成: 原状态=%v, 新状态=%v, ShutdownRequesting=%v",
		initialState, finalState, finalShutdownRequesting)
}

// ServiceStateNopEvent 是一个用于触发 ServiceState RunLoop 的空事件
type ServiceStateNopEvent struct{}

func (e *ServiceStateNopEvent) GetName() string {
	return "NopEvent"
}

func (e *ServiceStateNopEvent) Process(ctx context.Context, resource *ServiceState) {
	// 空实现，仅用于触发 RunLoop
}

// WithOfflineGracePeriodSec 设置 WorkerConfig 的离线优雅期
func WithOfflineGracePeriodSec(seconds int32) smgjson.ServiceConfigOption {
	return func(cfg *smgjson.ServiceConfigJson) {
		cfg.WorkerConfig.OfflineGracePeriodSec = smgjson.NewInt32Pointer(seconds)
	}
}
