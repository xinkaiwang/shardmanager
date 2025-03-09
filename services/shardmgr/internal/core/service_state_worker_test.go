package core

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
