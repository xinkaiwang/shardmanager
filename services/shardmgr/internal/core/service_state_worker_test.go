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
			safeAccessServiceState(ss, func(ss *ServiceState) {
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

			// 验证worker eph已正确设置到etcd
			ephItems := setup.FakeEtcd.List(ctx, ss.PathManager.GetWorkerEphPathPrefix(), 10)
			t.Logf("etcd中的worker eph节点数量: %d", len(ephItems))
			assert.Equal(t, 1, len(ephItems), "etcd应该包含1个worker eph节点")
			for _, item := range ephItems {
				t.Logf("etcd worker eph节点: 键=%s, 值=%s", item.Key, item.Value)
				assert.Equal(t, ephPath, item.Key, "worker eph节点路径应该正确")
				assert.Equal(t, string(workerEphJson), item.Value, "worker eph节点内容应该正确")
			}

			// 等待系统自动处理worker eph变化并创建worker state
			success, elapsedMs := waitForWorkerStateCreation(t, ss, "worker-1")
			t.Logf("等待worker-1创建结果: success=%v, 耗时=%dms", success, elapsedMs)
			assert.True(t, success, "应该在超时前成功创建worker state: worker-1, elapsedMs=%d ms", elapsedMs)

			// 验证worker state已创建
			safeAccessServiceState(ss, func(ss *ServiceState) {
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

			// 验证etcd中的worker eph节点
			ephItems = setup.FakeEtcd.List(ctx, ss.PathManager.GetWorkerEphPathPrefix(), 10)
			t.Logf("etcd中的worker eph节点数量: %d", len(ephItems))
			assert.Equal(t, 2, len(ephItems), "etcd应该包含2个worker eph节点")

			// 创建期望的键值对映射
			expectedEphMap := map[string]string{
				ephPath:  string(workerEphJson),
				eph2Path: string(worker2EphJson),
			}

			// 验证每个worker eph节点
			for _, item := range ephItems {
				t.Logf("etcd worker eph节点: 键=%s, 值长度=%d", item.Key, len(item.Value))
				expectedValue, exists := expectedEphMap[item.Key]
				assert.True(t, exists, "应该能在预期映射中找到键: %s", item.Key)
				assert.Equal(t, expectedValue, item.Value, "worker eph节点内容应该正确: %s", item.Key)
			}

			// 等待系统自动处理worker eph变化并创建worker state
			success, elapsedMs = waitForWorkerStateCreation(t, ss, "worker-2")
			t.Logf("等待worker-2创建结果: success=%v, 耗时=%dms", success, elapsedMs)
			assert.True(t, success, "应该在超时前成功创建worker state: worker-2")

			// 验证两个worker state都已创建
			safeAccessServiceState(ss, func(ss *ServiceState) {
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

			// 等待worker state被持久化到存储
			worker1StatePath := ss.PathManager.FmtWorkerStatePath(workerFullId)
			worker2StatePath := ss.PathManager.FmtWorkerStatePath(worker2FullId)

			// 等待worker-1状态持久化
			t.Logf("等待worker-1的state持久化到路径: %s", worker1StatePath)
			success1, waitDuration1 := waitForWorkerStatePersistence(t, setup.FakeStore, worker1StatePath)
			assert.True(t, success1, "worker-1状态应成功持久化")
			t.Logf("等待worker-1状态持久化结果: success=%v, 等待时间=%dms", success1, waitDuration1)

			// 等待worker-2状态持久化
			t.Logf("等待worker-2的state持久化到路径: %s", worker2StatePath)
			success2, waitDuration2 := waitForWorkerStatePersistence(t, setup.FakeStore, worker2StatePath)
			assert.True(t, success2, "worker-2状态应成功持久化")
			t.Logf("等待worker-2状态持久化结果: success=%v, 等待时间=%dms", success2, waitDuration2)

			// 验证FakeEtcdStore内容
			t.Log("验证FakeEtcdStore内容")
			storeData := setup.FakeStore.GetData()
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
		})
	})
}

// TestWorkerState_EphNodeLost 测试临时节点丢失时的状态转换
// 同时使用 safeAccessServiceState 确保对 ServiceState 的安全访问。
//
// 然而，测试仍会显示数据竞争警告，主要原因是底层 krunloop 库中的问题：
// - RunLoop.currentEventName 字段在主循环中被写入，同时被 RunloopSampler 在另一个 goroutine 中读取，没有同步保护
// - 这种数据竞争应该在 krunloop 包中通过使用互斥锁或原子操作来解决
// - 例如，可以修改 RunLoop 结构体，使用 atomic.Value 或 sync.RWMutex 保护 currentEventName 字段
func TestWorkerState_EphNodeLost(t *testing.T) {
	t.Run("worker ephemeral node lost", func(t *testing.T) {
		ctx := context.Background()

		originalLogger := klogging.GetDefaultLogger()
		nullLogger := klogging.NewLogrusLogger(ctx)
		klogging.SetDefaultLogger(nullLogger)
		// 测试结束时恢复原始日志记录器
		defer klogging.SetDefaultLogger(originalLogger)

		// 重置全局状态
		resetGlobalState(t)
		t.Logf("全局状态已重置")

		// 配置测试环境
		setup := CreateTestSetup(t)
		setup.SetupBasicConfig(t)
		t.Logf("测试环境已配置")

		// 初始化ServiceState并通过etcd触发worker状态变化
		etcdprov.RunWithEtcdProvider(setup.FakeEtcd, func() {
			shadow.RunWithEtcdStore(setup.FakeStore, func() {
				setup.CreateServiceState(t, 0)
				ss := setup.ServiceState
				t.Logf("ServiceState已初始化")

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
				assert.True(t, success, "应该在超时前成功创建worker state: worker-1")

				// 验证worker state创建成功且处于在线状态
				var workerState *WorkerState
				var workerStatePath string
				safeAccessServiceState(ss, func(ss *ServiceState) {
					worker, exists := ss.AllWorkers[workerFullId]
					assert.True(t, exists, "worker应该已创建")
					assert.Equal(t, data.WS_Online_healthy, worker.State, "worker应该处于在线状态")
					workerState = worker
					workerStatePath = ss.PathManager.FmtWorkerStatePath(workerFullId)
					t.Logf("worker初始状态验证完成: State=%v", worker.State)
				})

				// 记录初始状态
				initialState := workerState.State
				t.Logf("worker初始状态: %v", initialState)

				// 删除临时节点，触发临时节点丢失处理
				t.Logf("从etcd删除worker eph节点: %s", ephPath)
				setup.FakeEtcd.Delete(ctx, ephPath)
				t.Logf("已删除worker eph节点，等待状态同步")

				// 使用WaitUntil等待worker状态变化
				waitForStateCondition := func() (bool, string) {
					var state data.WorkerStateEnum
					var exists bool

					safeAccessServiceState(ss, func(ss *ServiceState) {
						worker, ok := ss.AllWorkers[workerFullId]
						if !ok {
							exists = false
							return
						}
						exists = true
						state = worker.State
					})

					if !exists {
						return false, "worker不存在"
					}

					// 检查状态是否变为离线状态
					isOffline := state == data.WS_Offline_graceful_period
					return isOffline, fmt.Sprintf("worker状态=%v, 是否离线=%v", state, isOffline)
				}

				t.Logf("等待worker状态从online_healthy变为offline_graceful_period")
				if !WaitUntil(t, waitForStateCondition, 1000, 50) {
					t.Fatalf("等待超时，状态仍未变化，测试失败")
				}

				t.Logf("worker状态已成功变为离线状态")

				// 验证最终状态
				var finalState data.WorkerStateEnum
				var gracePeriodTime int64

				safeAccessServiceState(ss, func(ss *ServiceState) {
					worker, exists := ss.AllWorkers[workerFullId]
					assert.True(t, exists, "worker应该仍然存在")
					finalState = worker.State
					gracePeriodTime = worker.GracePeriodStartTimeMs
				})

				assert.Equal(t, data.WS_Offline_graceful_period, finalState, "worker状态应变为WS_Offline_graceful_period")
				assert.True(t, gracePeriodTime > 0, "优雅期开始时间应被设置")
				t.Logf("worker状态已验证: 原状态=%v, 新状态=%v, GracePeriodStartTimeMs=%d",
					initialState, finalState, gracePeriodTime)

				// 等待worker state被持久化到存储
				t.Logf("等待worker state持久化到路径: %s", workerStatePath)
				success, waitDuration := waitForWorkerStatePersistence(t, setup.FakeStore, workerStatePath)
				assert.True(t, success, "worker状态应成功持久化")
				t.Logf("等待worker状态持久化结果: success=%v, 等待时间=%dms", success, waitDuration)

				// 验证持久化的状态内容
				storeData := setup.FakeStore.GetData()
				jsonStr := storeData[workerStatePath]
				storedWorkerState := smgjson.WorkerStateJsonFromJson(jsonStr)

				assert.NotNil(t, storedWorkerState, "应该能解析worker state json")
				assert.Equal(t, data.WorkerId("worker-1"), storedWorkerState.WorkerId, "持久化的WorkerId应正确")
				assert.Equal(t, data.WS_Offline_graceful_period, storedWorkerState.WorkerState,
					"持久化的状态应为WS_Offline_graceful_period")
				t.Logf("持久化数据验证完成: WorkerId=%s, State=%v",
					storedWorkerState.WorkerId, storedWorkerState.WorkerState)
			})
		})
	})
}

// TestWorkerShutdownRequest 测试通过worker eph节点请求关闭的流程
func TestWorkerShutdownRequest(t *testing.T) {
	ctx := context.Background()

	// 减少日志输出，使测试输出更清晰
	originalLogger := klogging.GetDefaultLogger()
	nullLogger := klogging.NewLogrusLogger(ctx)
	klogging.SetDefaultLogger(nullLogger)
	// 测试结束时恢复原始日志记录器
	defer klogging.SetDefaultLogger(originalLogger)

	// 重置全局状态
	resetGlobalState(t)
	t.Logf("全局状态已重置")

	// 配置测试环境
	setup := CreateTestSetup(t)
	setup.SetupBasicConfig(t)
	t.Logf("测试环境已配置")

	// 初始化ServiceState并通过etcd触发worker状态变化
	etcdprov.RunWithEtcdProvider(setup.FakeEtcd, func() {
		shadow.RunWithEtcdStore(setup.FakeStore, func() {
			setup.CreateServiceState(t, 0)
			ss := setup.ServiceState
			t.Logf("ServiceState已初始化")

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
			assert.True(t, success, "应该在超时前成功创建worker state: worker-1")

			// 验证初始状态
			var initialState data.WorkerStateEnum
			var initialShutdownRequesting bool
			workerStatePath := ""

			safeAccessServiceState(ss, func(ss *ServiceState) {
				worker, exists := ss.AllWorkers[workerFullId]
				assert.True(t, exists, "worker应该已创建")
				initialState = worker.State
				initialShutdownRequesting = worker.ShutdownRequesting
				workerStatePath = ss.PathManager.FmtWorkerStatePath(workerFullId)
				assert.Equal(t, data.WS_Online_healthy, worker.State, "worker初始状态应为healthy")
				assert.False(t, worker.ShutdownRequesting, "初始ShutdownRequesting应为false")
			})
			t.Logf("初始状态验证完成: 状态=%v, ShutdownRequesting=%v", initialState, initialShutdownRequesting)

			// 更新worker eph，设置关闭请求标志
			updatedWorkerEph := cougarjson.WorkerEphJsonFromJson(string(workerEphJson))
			updatedWorkerEph.ReqShutDown = 1 // 设置为true
			updatedWorkerEphJson, _ := json.Marshal(updatedWorkerEph)

			t.Logf("更新worker eph，设置ReqShutDown=1")
			// 更新etcd中的worker eph节点
			setup.FakeEtcd.Set(ctx, ephPath, string(updatedWorkerEphJson))
			t.Logf("已更新worker eph节点，等待状态同步")

			// 定义等待条件：等待worker状态变为shutdown_req
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

			// 等待状态变化
			t.Logf("等待worker状态变为online_shutdown_req")
			if !WaitUntil(t, waitForShutdownReq, 1000, 50) {
				t.Fatalf("等待超时，worker状态未变为shutdown_req")
			}
			t.Logf("worker状态已成功变为shutdown_req")

			// 验证最终状态
			var finalState data.WorkerStateEnum
			var finalShutdownRequesting bool

			safeAccessServiceState(ss, func(ss *ServiceState) {
				worker, exists := ss.AllWorkers[workerFullId]
				assert.True(t, exists, "worker应该存在")
				finalState = worker.State
				finalShutdownRequesting = worker.ShutdownRequesting
			})

			assert.Equal(t, data.WS_Online_shutdown_req, finalState, "worker状态应变为WS_Online_shutdown_req")
			assert.True(t, finalShutdownRequesting, "ShutdownRequesting应为true")
			t.Logf("最终状态验证完成: 原状态=%v, 新状态=%v, ShutdownRequesting=%v",
				initialState, finalState, finalShutdownRequesting)

			// 验证状态持久化
			t.Logf("等待worker state持久化到路径: %s", workerStatePath)
			success, waitDuration := waitForWorkerStatePersistence(t, setup.FakeStore, workerStatePath)
			assert.True(t, success, "worker状态应成功持久化")
			t.Logf("等待worker状态持久化结果: success=%v, 等待时间=%dms", success, waitDuration)

			// 验证持久化的内容
			storeData := setup.FakeStore.GetData()
			jsonStr := storeData[workerStatePath]
			storedWorkerState := smgjson.WorkerStateJsonFromJson(jsonStr)

			assert.NotNil(t, storedWorkerState, "应该能解析worker state json")
			assert.Equal(t, data.WorkerId("worker-1"), storedWorkerState.WorkerId, "持久化的WorkerId应正确")
			assert.Equal(t, data.WS_Online_shutdown_req, storedWorkerState.WorkerState,
				"持久化的状态应为WS_Online_shutdown_req")
			t.Logf("持久化数据验证完成: WorkerId=%s, State=%v",
				storedWorkerState.WorkerId, storedWorkerState.WorkerState)
			// assert.Equal(t, true, false, "测试失败")
		})
	})
}
