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

			// 验证FakeEtcdStore内容
			t.Log("验证FakeEtcdStore内容")
			storeData := setup.FakeStore.GetData()
			t.Logf("FakeEtcdStore数据条数: %d", len(storeData))

			// 验证worker state是否被持久化
			workerStatePrefix := ss.PathManager.GetWorkerStatePathPrefix()
			workerStatePersisted := false

			for k, v := range storeData {
				if strings.HasPrefix(k, workerStatePrefix) {
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
func TestWorkerState_EphNodeLost(t *testing.T) {
	ctx := context.Background()

	// 重置全局状态
	resetGlobalState(t)
	t.Logf("全局状态已重置")

	// 配置测试环境
	setup := CreateTestSetup(t)
	setup.SetupBasicConfig(t)
	t.Logf("测试环境已配置")

	// 初始化ServiceState并创建一个worker
	etcdprov.RunWithEtcdProvider(setup.FakeEtcd, func() {
		shadow.RunWithEtcdStore(setup.FakeStore, func() {
			setup.CreateServiceState(t, 0)
			ss := setup.ServiceState
			t.Logf("ServiceState已初始化")

			// 检查 StoreProvider 的类型和配置
			t.Logf("ServiceState.StoreProvider类型: %T", ss.StoreProvider)
			t.Logf("setup.FakeStore.GetPutCalledCount初始值: %d", setup.FakeStore.GetPutCalledCount())

			// 验证 GetCurrentEtcdStore 是否指向 FakeStore
			currentStore := shadow.GetCurrentEtcdStore(context.Background())
			t.Logf("当前 EtcdStore 类型: %T, 是否等于FakeStore: %v", currentStore, currentStore == setup.FakeStore)

			// 创建在线状态的worker
			workerFullId := data.NewWorkerFullId("worker-1", "session-1", ss.IsStateInMemory())
			workerState := NewWorkerState(workerFullId.WorkerId, workerFullId.SessionId)
			safeAccessServiceState(ss, func(ss *ServiceState) {
				ss.AllWorkers[workerFullId] = workerState
				t.Logf("已创建处于在线状态的worker: %s", workerFullId.String())
			})

			// 测试WS_Online_healthy状态下临时节点丢失
			safeAccessServiceState(ss, func(ss *ServiceState) {
				assert.Equal(t, data.WS_Online_healthy, workerState.State, "初始状态应为WS_Online_healthy")
				assert.Equal(t, int64(0), workerState.GracePeriodStartTimeMs, "优雅期开始时间应为0")
				t.Logf("初始状态验证完成: State=%v, GracePeriodStartTimeMs=%d",
					workerState.State, workerState.GracePeriodStartTimeMs)

				// 测试前记录状态
				beforePutCount := setup.FakeStore.GetPutCalledCount()
				workerStatePath := ss.PathManager.FmtWorkerStatePath(workerFullId)
				t.Logf("调用前状态: 存储路径=%s, PutCount=%d", workerStatePath, beforePutCount)
				beforeData := setup.FakeStore.GetData()
				t.Logf("调用前存储数据: %+v", beforeData)

				// 模拟临时节点丢失
				workerState.onEphNodeLost(ctx, ss)
				t.Logf("已调用onEphNodeLost处理临时节点丢失")

				// 验证状态转换
				assert.Equal(t, data.WS_Offline_graceful_period, workerState.State,
					"状态应转换为WS_Offline_graceful_period")
				assert.True(t, workerState.GracePeriodStartTimeMs > 0,
					"优雅期开始时间应被设置")
				t.Logf("状态转换验证完成: State=%v, GracePeriodStartTimeMs=%d",
					workerState.State, workerState.GracePeriodStartTimeMs)

				// 检查Put调用计数变化
				afterPutCount := setup.FakeStore.GetPutCalledCount()
				t.Logf("调用后状态: PutCount=%d, 增加=%d", afterPutCount, afterPutCount-beforePutCount)

				// 等待worker状态被持久化
				success, waitDuration := waitForWorkerStatePersistence(t, setup.FakeStore, workerStatePath)
				assert.True(t, success, "worker状态应成功持久化，路径: %s", workerStatePath)
				t.Logf("等待worker状态持久化结果: success=%v, 等待时间=%dms", success, waitDuration)

				if success {
					// 获取持久化的数据
					storeData := setup.FakeStore.GetData()
					jsonStr := storeData[workerStatePath]
					storedWorkerState := decodeWorkerStateJson(t, jsonStr)

					// 验证持久化数据
					assert.NotNil(t, storedWorkerState, "应该能解析worker state json")
					assert.Equal(t, workerFullId.WorkerId, storedWorkerState.WorkerId, "持久化的WorkerId应正确")
					assert.Equal(t, data.WS_Offline_graceful_period, storedWorkerState.WorkerState, "持久化的状态应为WS_Offline_graceful_period")
					t.Logf("持久化数据验证完成: WorkerId=%s, State=%v",
						storedWorkerState.WorkerId, storedWorkerState.WorkerState)
				}
			})

			// 重置状态并测试WS_Online_shutdown_req状态下临时节点丢失
			safeAccessServiceState(ss, func(ss *ServiceState) {
				workerState.State = data.WS_Online_shutdown_req
				workerState.GracePeriodStartTimeMs = 0
				t.Logf("已重置worker状态为WS_Online_shutdown_req")

				workerState.onEphNodeLost(ctx, ss)
				t.Logf("已调用onEphNodeLost处理临时节点丢失")

				assert.Equal(t, data.WS_Offline_graceful_period, workerState.State,
					"状态应转换为WS_Offline_graceful_period")
				assert.True(t, workerState.GracePeriodStartTimeMs > 0,
					"优雅期开始时间应被设置")
				t.Logf("状态转换验证完成: State=%v, GracePeriodStartTimeMs=%d",
					workerState.State, workerState.GracePeriodStartTimeMs)
			})

			// 重置状态并测试WS_Online_shutdown_hat状态下临时节点丢失
			safeAccessServiceState(ss, func(ss *ServiceState) {
				workerState.State = data.WS_Online_shutdown_hat
				workerState.GracePeriodStartTimeMs = 0
				t.Logf("已重置worker状态为WS_Online_shutdown_hat")

				workerState.onEphNodeLost(ctx, ss)
				t.Logf("已调用onEphNodeLost处理临时节点丢失")

				assert.Equal(t, data.WS_Offline_draining_hat, workerState.State,
					"状态应转换为WS_Offline_draining_hat")
				t.Logf("状态转换验证完成: State=%v", workerState.State)
			})

			// 重置状态并测试WS_Online_shutdown_permit状态下临时节点丢失
			safeAccessServiceState(ss, func(ss *ServiceState) {
				workerState.State = data.WS_Online_shutdown_permit
				workerState.GracePeriodStartTimeMs = 0
				t.Logf("已重置worker状态为WS_Online_shutdown_permit")

				workerState.onEphNodeLost(ctx, ss)
				t.Logf("已调用onEphNodeLost处理临时节点丢失")

				assert.Equal(t, data.WS_Offline_dead, workerState.State,
					"状态应转换为WS_Offline_dead")
				assert.True(t, workerState.GracePeriodStartTimeMs > 0,
					"优雅期开始时间应被设置")
				t.Logf("状态转换验证完成: State=%v, GracePeriodStartTimeMs=%d",
					workerState.State, workerState.GracePeriodStartTimeMs)
			})

			// 注意：由于klogging.Fatal难以在测试中捕获，此处不测试已离线状态下临时节点丢失的情况
			// 在实际环境中，这类情况会触发Fatal错误
		})
	})
}

/*
func TestWorkerState_Timeout(t *testing.T) {
	ctx := context.Background()

	// 重置全局状态
	resetGlobalState(t)
	t.Logf("全局状态已重置")

	// 配置测试环境
	setup := CreateTestSetup(t)
	setup.SetupBasicConfig(t)
	t.Logf("测试环境已配置")

	// 初始化ServiceState并创建一个worker
	etcdprov.RunWithEtcdProvider(setup.FakeEtcd, func() {
		shadow.RunWithEtcdStore(setup.FakeStore, func() {
			setup.CreateServiceState(t, 0)
			ss := setup.ServiceState
			t.Logf("ServiceState已初始化")

			// 配置服务，设置优雅期为5秒
			ss.ServiceConfig.WorkerConfig.OfflineGracePeriodSec = 5
			t.Logf("已设置优雅期为5秒: OfflineGracePeriodSec=%d",
				ss.ServiceConfig.WorkerConfig.OfflineGracePeriodSec)

			// 创建WS_Offline_graceful_period状态的worker
			workerFullId := data.NewWorkerFullId("worker-1", "session-1", ss.IsStateInMemory())
			workerState := NewWorkerState(workerFullId.WorkerId, workerFullId.SessionId)
			workerState.State = data.WS_Offline_graceful_period
			// 设置优雅期开始时间为6秒前（确保已过期）
			workerState.GracePeriodStartTimeMs = kcommon.GetWallTimeMs() - 6*1000
			t.Logf("已创建处于优雅期状态的worker: %s, 优雅期开始于%d毫秒前",
				workerFullId.String(), 6*1000)

			safeAccessServiceState(ss, func(ss *ServiceState) {
				ss.AllWorkers[workerFullId] = workerState
				t.Logf("已将worker添加到AllWorkers")

				// 测试优雅期超时
				needsDelete := workerState.checkWorkerForTimeout(ctx, ss)
				t.Logf("已调用checkWorkerForTimeout检查超时")

				// 验证状态转换
				assert.Equal(t, data.WS_Offline_draining_candidate, workerState.State,
					"优雅期超时后状态应转换为WS_Offline_draining_candidate")
				assert.False(t, needsDelete, "needsDelete应为false")
				t.Logf("优雅期超时验证完成: State=%v, needsDelete=%v",
					workerState.State, needsDelete)

				// 验证持久化调用
				storeData := setup.FakeStore.GetData()
				workerStatePath := ss.PathManager.FmtWorkerStatePath(workerFullId)
				v, exists := storeData[workerStatePath]
				assert.True(t, exists, "Worker状态应被持久化")

				if exists {
					var workerStateJson smgjson.WorkerStateJson
					err := json.Unmarshal([]byte(v), &workerStateJson)
					assert.NoError(t, err, "应该能解析worker state json")

					if err == nil {
						assert.Equal(t, string(data.WS_Offline_draining_candidate), string(workerStateJson.WorkerState),
							"持久化的状态应为WS_Offline_draining_candidate")
						t.Logf("持久化数据验证完成: State=%v", workerStateJson.WorkerState)
					}
				}
			})

			// 测试WS_Offline_dead状态超时
			safeAccessServiceState(ss, func(ss *ServiceState) {
				workerState.State = data.WS_Offline_dead
				workerState.GracePeriodStartTimeMs = kcommon.GetWallTimeMs() - 21*1000 // 21秒前
				t.Logf("已重置worker状态为WS_Offline_dead，死亡时间开始于%d毫秒前", 21*1000)

				needsDelete := workerState.checkWorkerForTimeout(ctx, ss)
				t.Logf("已调用checkWorkerForTimeout检查超时")

				// 验证状态转换
				assert.Equal(t, data.WS_Deleted, workerState.State,
					"死亡状态超时后应转换为WS_Deleted")
				assert.True(t, needsDelete, "needsDelete应为true")
				t.Logf("死亡状态超时验证完成: State=%v, needsDelete=%v",
					workerState.State, needsDelete)
			})
		})
	})
}

func TestWorkerState_UpdateByEph(t *testing.T) {
	ctx := context.Background()

	// 重置全局状态
	resetGlobalState(t)
	t.Logf("全局状态已重置")

	// 配置测试环境
	setup := CreateTestSetup(t)
	setup.SetupBasicConfig(t)
	t.Logf("测试环境已配置")

	// 初始化ServiceState并创建一个worker
	etcdprov.RunWithEtcdProvider(setup.FakeEtcd, func() {
		shadow.RunWithEtcdStore(setup.FakeStore, func() {
			setup.CreateServiceState(t, 0)
			ss := setup.ServiceState
			t.Logf("ServiceState已初始化")

			// 创建worker
			workerFullId := data.NewWorkerFullId("worker-1", "session-1", ss.IsStateInMemory())
			workerState := NewWorkerState(workerFullId.WorkerId, workerFullId.SessionId)
			t.Logf("已创建worker: %s", workerFullId.String())

			// 创建初始临时节点数据
			workerEph := cougarjson.NewWorkerEphJson("worker-1", "session-1", 1234567890000, 60)
			workerEph.AddressPort = "localhost:8080"
			t.Logf("已创建临时节点数据，地址端口: %s", workerEph.AddressPort)

			safeAccessServiceState(ss, func(ss *ServiceState) {
				ss.AllWorkers[workerFullId] = workerState
				t.Logf("已将worker添加到AllWorkers")

				// 测试初始更新
				dirty := workerState.updateWorkerByEph(ctx, workerEph)
				t.Logf("已调用updateWorkerByEph进行初始更新")

				// 验证更新结果
				assert.True(t, dirty.IsDirty(), "初始更新应标记为dirty")
				assert.Equal(t, "localhost:8080", workerState.WorkerInfo.AddressPort,
					"WorkerInfo应正确更新")
				assert.Equal(t, 0, len(workerState.WorkerReportedAssignments),
					"初始应无任务分配")
				t.Logf("初始更新验证完成: dirty=%v, AddressPort=%s, ReportedAssignments数量=%d",
					dirty.IsDirty(), workerState.WorkerInfo.AddressPort, len(workerState.WorkerReportedAssignments))
			})

			// 测试添加任务分配
			safeAccessServiceState(ss, func(ss *ServiceState) {
				// 添加任务到临时节点
				assignment := cougarjson.NewAssignmentJson("shard-1", 0, "assignment-1", cougarjson.CAS_Active)
				workerEph.Assignments = append(workerEph.Assignments, assignment)
				t.Logf("已添加任务到临时节点: ShardId=%s, AssignmentId=%s",
					assignment.ShardId, assignment.AsginmentId)

				dirty := workerState.updateWorkerByEph(ctx, workerEph)
				t.Logf("已调用updateWorkerByEph更新任务分配")

				// 验证任务分配更新
				assert.True(t, dirty.IsDirty(), "任务变更应标记为dirty")
				assert.Equal(t, 1, len(workerState.WorkerReportedAssignments),
					"应有1个任务分配")
				_, exists := workerState.WorkerReportedAssignments[data.AssignmentId("assignment-1")]
				assert.True(t, exists, "应包含特定任务分配")
				t.Logf("任务分配更新验证完成: dirty=%v, ReportedAssignments数量=%d, 包含assignment-1=%v",
					dirty.IsDirty(), len(workerState.WorkerReportedAssignments), exists)
			})

			// 测试关闭请求
			safeAccessServiceState(ss, func(ss *ServiceState) {
				// 设置关闭请求
				workerEph.ReqShutDown = 1
				t.Logf("已在临时节点设置关闭请求: ReqShutDown=%d", workerEph.ReqShutDown)

				dirty := workerState.updateWorkerByEph(ctx, workerEph)
				t.Logf("已调用updateWorkerByEph处理关闭请求")

				// 验证状态变更
				assert.True(t, dirty.IsDirty(), "关闭请求应标记为dirty")
				assert.True(t, workerState.ShutdownRequesting, "ShutdownRequesting应为true")
				assert.Equal(t, data.WS_Online_shutdown_req, workerState.State,
					"状态应变为WS_Online_shutdown_req")
				t.Logf("关闭请求验证完成: dirty=%v, ShutdownRequesting=%v, State=%v",
					dirty.IsDirty(), workerState.ShutdownRequesting, workerState.State)
			})

			// 测试无变更场景
			safeAccessServiceState(ss, func(ss *ServiceState) {
				dirty := workerState.updateWorkerByEph(ctx, workerEph)
				t.Logf("已调用updateWorkerByEph处理无变更场景")

				// 验证无变更
				assert.False(t, dirty.IsDirty(), "无变更时不应标记为dirty")
				t.Logf("无变更场景验证完成: dirty=%v", dirty.IsDirty())
			})
		})
	})
}
*/

// decodeWorkerStateJson 将JSON字符串解析为WorkerStateJson对象
func decodeWorkerStateJson(t *testing.T, jsonStr string) *smgjson.WorkerStateJson {
	var workerStateJson smgjson.WorkerStateJson
	err := json.Unmarshal([]byte(jsonStr), &workerStateJson)
	if err != nil {
		t.Logf("无法解析WorkerStateJson: %v", err)
		return nil
	}
	return &workerStateJson
}

// waitForWorkerStatePersistence 等待特定路径的worker状态被持久化到存储中
func waitForWorkerStatePersistence(t *testing.T, store *shadow.FakeEtcdStore, path string) (bool, int64) {
	t.Helper()

	startMs := kcommon.GetMonoTimeMs()
	fn := func() (bool, string) {
		// 获取存储数据
		storeData := store.GetData()

		// 检查目标路径是否存在
		value, exists := storeData[path]

		// 生成调试信息
		debugInfo := fmt.Sprintf("路径=%s, 存在=%v, 数据长度=%d, 总存储项数=%d",
			path, exists, len(value), len(storeData))

		// 记录当前尝试的状态
		t.Logf("检查worker状态持久化: %s", debugInfo)

		return exists, debugInfo
	}

	// 使用已有的WaitUntil函数实现等待，最多等待1秒，每50ms检查一次
	succ := WaitUntil(t, fn, 1000, 50)
	return succ, kcommon.GetMonoTimeMs() - startMs
}
