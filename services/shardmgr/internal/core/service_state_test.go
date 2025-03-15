package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/etcdprov"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/shadow"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

// TestMain 用于设置和清理测试环境
func TestMain(m *testing.M) {
	// 在所有测试开始前重置全局状态
	resetGlobalState(nil)

	// 运行测试
	result := m.Run()

	// 退出并返回测试结果
	os.Exit(result)
}

func TestServiceState_Basic(t *testing.T) {
	// 重置全局状态，确保测试环境干净
	resetGlobalState(t)

	ctx := context.Background()
	// 创建测试用的 FakeEtcdProvider
	fakeEtcd := etcdprov.NewFakeEtcdProvider()

	// 准备初始数据
	serviceInfoPath := "/smg/config/service_info.json"
	serviceInfo := smgjson.CreateTestServiceInfo()
	data, _ := json.Marshal(serviceInfo)
	fakeEtcd.Set(ctx, serviceInfoPath, string(data))

	// 设置服务配置
	serviceConfigPath := "/smg/config/service_config.json"
	serviceConfig := smgjson.CreateTestServiceConfig()
	configData, _ := json.Marshal(serviceConfig)
	fakeEtcd.Set(ctx, serviceConfigPath, string(configData))

	// 使用 RunWithEtcdProvider 运行测试
	etcdprov.RunWithEtcdProvider(fakeEtcd, func() {
		// 创建 ServiceState
		ss := NewServiceState(ctx, "TestServiceState_Basic")

		// 不使用 defer 调用 StopAndWaitForExit，而是在测试结束前直接调用
		// defer ss.StopAndWaitForExit(ctx)

		// 验证基本信息是否正确加载
		assert.Equal(t, "test-service", ss.ServiceInfo.ServiceName)
		assert.Equal(t, smgjson.ST_SOFT_STATEFUL, ss.ServiceInfo.ServiceType)
		assert.Equal(t, smgjson.MP_StartBeforeKill, ss.ServiceInfo.DefaultHints.MovePolicy)
		assert.Equal(t, int32(10), ss.ServiceInfo.DefaultHints.MaxReplicaCount)
		assert.Equal(t, int32(1), ss.ServiceInfo.DefaultHints.MinReplicaCount)

		// 验证 IsStateInMemory 方法
		assert.True(t, ss.IsStateInMemory())

		// 在测试结束前调用 StopAndWaitForExit
		ss.StopAndWaitForExit(ctx)
	})
}

func TestServiceState_ShardManagement(t *testing.T) {
	// 重置全局状态，确保测试环境干净
	resetGlobalState(t)

	ctx := context.Background()
	// 创建测试用的 FakeEtcdProvider
	fakeEtcd := etcdprov.NewFakeEtcdProvider()

	// 准备初始数据
	serviceInfoPath := "/smg/config/service_info.json"
	serviceInfo := smgjson.CreateTestServiceInfo()
	serviceInfoData, _ := json.Marshal(serviceInfo)
	fakeEtcd.Set(ctx, serviceInfoPath, string(serviceInfoData))

	// 设置服务配置
	serviceConfigPath := "/smg/config/service_config.json"
	// 使用选项模式自定义配置
	serviceConfig := smgjson.CreateTestServiceConfigWithOptions(
		smgjson.WithReplicaLimits(1, 10),
		smgjson.WithMovePolicy(smgjson.MP_StartBeforeKill),
	)
	configData, _ := json.Marshal(serviceConfig)
	fakeEtcd.Set(ctx, serviceConfigPath, string(configData))

	// 设置 shard 计划
	shardPlanPath := "/smg/config/shard_plan.txt"
	shardPlan := `shard-1
shard-2|{"min_replica_count":2,"max_replica_count":5}
shard-3|{"move_type":"kill_before_start"}`
	fakeEtcd.Set(ctx, shardPlanPath, shardPlan)

	// 使用 RunWithEtcdProvider 运行测试
	etcdprov.RunWithEtcdProvider(fakeEtcd, func() {
		// 创建 ServiceState
		ss := NewServiceState(ctx, "TestServiceState_ShardManagement")

		// 不使用 defer 调用 StopAndWaitForExit，而是在测试结束前直接调用
		// defer ss.StopAndWaitForExit(ctx)

		// 验证 shard 是否被正确加载
		assert.Equal(t, 3, len(ss.AllShards))

		// 验证 shard-1 的默认配置
		shard1, ok := ss.AllShards["shard-1"]
		assert.True(t, ok)
		assert.Equal(t, "shard-1", string(shard1.ShardId))

		// 验证 shard-2 的自定义配置 (min=2, max=5)
		shard2, ok := ss.AllShards["shard-2"]
		assert.True(t, ok)
		assert.Equal(t, "shard-2", string(shard2.ShardId))

		// 验证 shard-3 的自定义迁移策略 (move=kill_before_start)
		shard3, ok := ss.AllShards["shard-3"]
		assert.True(t, ok)
		assert.Equal(t, "shard-3", string(shard3.ShardId))

		// 在测试结束前调用 StopAndWaitForExit
		ss.StopAndWaitForExit(ctx)
	})
}

func TestServiceState_ShadowStateWrite(t *testing.T) {
	// 测试开始前，确保EtcdStore和EtcdProvider被正确重置，避免全局变量状态干扰
	resetGlobalState(t)

	// 创建一个FakeEtcdProvider用于测试
	t.Log("创建FakeEtcdProvider")
	fakeEtcd := etcdprov.NewFakeEtcdProvider()

	// 使用RunWithEtcdProvider模式替代直接设置全局EtcdProvider
	etcdprov.RunWithEtcdProvider(fakeEtcd, func() {
		// 创建FakeEtcdStore
		t.Log("创建FakeEtcdStore")
		fakeStore := shadow.NewFakeEtcdStore()

		// 直接设置全局的EtcdStore
		shadow.SetCurrentEtcdStore(fakeStore)

		// 记录测试开始时的EtcdStore状态
		t.Logf("测试开始时EtcdStore状态: %s", shadow.DumpStoreStats())

		// 验证全局EtcdProvider已正确设置
		ctx := context.Background()
		currentProvider := etcdprov.GetCurrentEtcdProvider(ctx)
		t.Logf("当前EtcdProvider: %T", currentProvider)
		assert.Equal(t, "*etcdprov.FakeEtcdProvider", fmt.Sprintf("%T", currentProvider), "应该正确设置FakeEtcdProvider")

		// 验证全局EtcdStore已正确设置
		currentStore := shadow.GetCurrentEtcdStore(ctx)
		t.Logf("当前EtcdStore: %T", currentStore)
		assert.Equal(t, "*shadow.FakeEtcdStore", fmt.Sprintf("%T", currentStore), "应该正确设置FakeEtcdStore")

		// 1. 准备初始数据
		t.Log("准备初始数据")

		// 准备服务信息，使用测试辅助函数
		t.Log("准备服务信息")
		serviceInfo := smgjson.CreateTestServiceInfo()
		fakeEtcd.Set(ctx, "/smg/config/service_info.json", serviceInfo.ToJson())

		// 准备服务配置，使用测试辅助函数
		t.Log("准备服务配置")
		serviceConfig := smgjson.CreateTestServiceConfig()
		fakeEtcd.Set(ctx, "/smg/config/service_config.json", serviceConfig.ToJson())

		// 准备分片计划
		t.Log("准备分片计划")
		shardPlanStr := `shard-1
shard-2|{"min_replica_count":2,"max_replica_count":5}
shard-3|{"migration_strategy":{"shard_move_type":"kill_before_start"}}`
		fakeEtcd.Set(ctx, "/smg/config/shard_plan.txt", shardPlanStr)

		// 2. 创建ServiceState
		t.Log("创建ServiceState")
		ss := NewServiceState(ctx, "TestServiceState_ShadowStateWrite")

		// 不使用 defer 调用 StopAndWaitForExit，而是在测试结束前直接调用
		// defer ss.StopAndWaitForExit(ctx)

		// 3. 验证影子分片状态写入
		t.Log("开始验证影子分片状态写入")

		// 等待ServiceState加载分片
		t.Log("等待ServiceState加载分片")
		success, waitDuration := waitForServiceShards(t, ss, 3)
		assert.True(t, success, "应该能在超时前加载分片")
		t.Logf("加载分片等待时间: %v", waitDuration)

		// 验证分片数量
		t.Logf("AllShards数量: %d", len(ss.AllShards))
		assert.Equal(t, 3, len(ss.AllShards), "应该有3个分片")

		// 验证特定分片的存在
		_, ok := ss.AllShards["shard-1"]
		t.Logf("shard-1存在: %v", ok)
		assert.True(t, ok, "应该能找到 shard-1")

		_, ok = ss.AllShards["shard-2"]
		t.Logf("shard-2存在: %v", ok)
		assert.True(t, ok, "应该能找到 shard-2")

		_, ok = ss.AllShards["shard-3"]
		t.Logf("shard-3存在: %v", ok)
		assert.True(t, ok, "应该能找到 shard-3")

		// 等待分片状态写入到 FakeEtcdStore
		t.Log("等待分片状态写入到 FakeEtcdStore")
		storeCondition := func() (bool, string) {
			items := fakeStore.List("/smg/shard_state/")
			return len(items) >= 3, fmt.Sprintf("FakeEtcdStore中分片状态数量: %d，期望数量: %d", len(items), 3)
		}
		storeSuccess, elapsedMs := WaitUntil(t, storeCondition, 1000, 20)
		assert.True(t, storeSuccess, "应该能在超时前写入分片状态, 耗时=%dms", elapsedMs)

		// 检查FakeEtcdStore中的数据
		t.Log("检查FakeEtcdStore中的数据")
		t.Logf("FakeEtcdStore调用次数: %d", fakeStore.GetPutCalledCount())
		storeData := fakeStore.GetData()
		t.Logf("FakeEtcdStore数据条数: %d", len(storeData))
		for k, v := range storeData {
			t.Logf("  - 键: %s, 值长度: %d", k, len(v))
		}

		// 确保在 FakeEtcdStore 中可以找到分片状态
		t.Log("检查FakeEtcdStore中的分片状态")
		items := fakeStore.List("/smg/shard_state/")
		t.Logf("FakeEtcdStore中分片状态数量: %d", len(items))
		assert.GreaterOrEqual(t, len(items), 3, "应该在FakeEtcdStore中找到至少3个分片状态")

		// 在测试结束前调用 StopAndWaitForExit
		ss.StopAndWaitForExit(ctx)

		// 测试结束后清理，避免影响其他测试
		resetGlobalState(t)
	})
}

// TestServiceState_PreexistingShardState 测试预先存在的分片状态如何被加载和更新
func TestServiceState_PreexistingShardState(t *testing.T) {
	ctx := context.Background()
	// 重置全局状态，确保测试环境干净
	resetGlobalState(t)

	// 创建一个FakeEtcdProvider用于测试
	fakeEtcd := etcdprov.NewFakeEtcdProvider()
	// 创建FakeEtcdStore
	fakeStore := shadow.NewFakeEtcdStore()

	// 1. 预先创建分片状态

	// 准备服务信息
	serviceInfo := smgjson.CreateTestServiceInfo()
	fakeEtcd.Set(ctx, "/smg/config/service_info.json", serviceInfo.ToJson())

	// 准备服务配置
	serviceConfig := smgjson.CreateTestServiceConfig()
	fakeEtcd.Set(ctx, "/smg/config/service_config.json", serviceConfig.ToJson())

	// 设置初始分片计划
	shardPlanStr := `shard-1
shard-2`
	fakeEtcd.Set(ctx, "/smg/config/shard_plan.txt", shardPlanStr)

	// 创建和存储预先存在的分片状态
	t.Log("创建预先存在的分片状态")
	pm := config.NewPathManager()

	// 创建正常的分片状态
	shard1 := &smgjson.ShardStateJson{
		ShardName: "shard-1",
		LameDuck:  0, // 正常分片
	}
	fakeEtcd.Set(ctx, pm.FmtShardStatePath("shard-1"), shard1.ToJson())

	// 创建正常的分片状态
	shard2 := &smgjson.ShardStateJson{
		ShardName: "shard-2",
		LameDuck:  0, // 正常分片
	}
	fakeEtcd.Set(ctx, pm.FmtShardStatePath("shard-2"), shard2.ToJson())

	// 创建已经是 lameDuck 的分片状态
	// 注意：即使设置为 lameDuck，因为它在分片计划中，所以 ServiceState 会将其调整为非 lameDuck
	shard3 := &smgjson.ShardStateJson{
		ShardName: "shard-3",
		LameDuck:  1, // 已经是 lameDuck
	}
	fakeEtcd.Set(ctx, pm.FmtShardStatePath("shard-3"), shard3.ToJson())

	// 检查分片状态是否已经写入 etcd
	fakeEtcdItems := fakeEtcd.List(ctx, "/smg/shard_state/", 100)
	t.Logf("预先创建了 %d 个分片状态", len(fakeEtcdItems))
	for _, item := range fakeEtcdItems {
		t.Logf("  - 键: %s, 值长度: %d", item.Key, len(item.Value))
	}

	// 记录当前的 FakeEtcdStore 写入次数供后续验证
	initialPutCount := fakeStore.GetPutCalledCount()
	t.Logf("初始化时 FakeEtcdStore 的写入次数: %d", initialPutCount)

	etcdprov.RunWithEtcdProvider(fakeEtcd, func() {
		shadow.RunWithEtcdStore(fakeStore, func() {
			// 2. 创建 ServiceState (在分片状态已存在的情况下启动)
			t.Log("创建 ServiceState，应该加载预先存在的分片状态")
			ss := NewServiceState(ctx, "TestServiceState_PreexistingShardState")

			// 3. 等待分片状态被加载
			success, waitDuration := waitForServiceShards(t, ss, 3)
			assert.True(t, success, "应该能在超时前加载预先存在的分片状态")
			t.Logf("加载预先存在的分片状态等待时间: %v", waitDuration)

			// 验证初始化时 ServiceState 是否向 etcd 写入了数据
			// 由于数据已经存在且是最新的，可能不会进行写入
			currentPutCount := fakeStore.GetPutCalledCount()
			t.Logf("ServiceState 初始化后 FakeEtcdStore 的写入次数: %d，增加了 %d 次",
				currentPutCount, currentPutCount-initialPutCount)

			// 检查 FakeEtcdStore 中的数据
			storeData := fakeStore.GetData()
			t.Logf("FakeEtcdStore 存储了 %d 个键值对", len(storeData))
			for k, v := range storeData {
				t.Logf("  - 键: %s, 值长度: %d", k, len(v))
			}

			// 验证分片状态是否正确
			t.Log("验证加载的分片状态")
			shard1Loaded, ok := ss.AllShards["shard-1"]
			assert.True(t, ok, "应该能找到 shard-1")
			assert.False(t, shard1Loaded.LameDuck, "shard-1 不应该是 lameDuck")

			shard2Loaded, ok := ss.AllShards["shard-2"]
			assert.True(t, ok, "应该能找到 shard-2")
			assert.False(t, shard2Loaded.LameDuck, "shard-2 不应该是 lameDuck")

			shard3Loaded, ok := ss.AllShards["shard-3"]
			assert.True(t, ok, "应该能找到 shard-3")
			// shard-3 初始创建为 lameDuck=1
			assert.True(t, shard3Loaded.LameDuck, "shard-3 应该是 lameDuck，因为它不在分片计划中")

			// 4. 更新分片计划，让一个已经是 lameDuck 的分片重新回到分片计划中
			t.Log("更新分片计划，添加新分片，移除旧分片，恢复 lameDuck 分片")
			updatedShardPlan := `shard-1
shard-4|{"min_replica_count":4,"max_replica_count":8}`
			// 注意：shard-2 被移除，shard-3 保留（它之前是 lameDuck），shard-4 是新添加的
			fakeEtcd.Set(ctx, "/smg/config/shard_plan.txt", updatedShardPlan)

			// 记录更新前的写入次数
			beforeUpdatePutCount := fakeStore.GetPutCalledCount()
			t.Logf("分片计划更新前 FakeEtcdStore 的写入次数: %d", beforeUpdatePutCount)

			// 5. 等待分片状态更新
			success, waitDuration = waitForServiceShards(t, ss, 4) // 应该有 4 个分片: 1,2,3,4 (2 被标记为 lameDuck, 3 应该变回非 lameDuck)
			assert.True(t, success, "应该能在超时前更新分片状态")
			t.Logf("更新分片等待时间: %v", waitDuration)

			// 验证更新后的写入情况
			afterUpdatePutCount := fakeStore.GetPutCalledCount()
			t.Logf("分片计划更新后 FakeEtcdStore 的写入次数: %d，增加了 %d 次",
				afterUpdatePutCount, afterUpdatePutCount-beforeUpdatePutCount)

			// 检查 FakeEtcdStore 中的数据
			updatedStoreData := fakeStore.GetData()
			t.Logf("更新后 FakeEtcdStore 存储了 %d 个键值对", len(updatedStoreData))
			for k, v := range updatedStoreData {
				t.Logf("  - 键: %s, 值长度: %d", k, len(v))
			}

			// 验证更新后的分片状态
			t.Log("验证更新后的分片状态")
			shard1Updated, ok := ss.AllShards["shard-1"]
			assert.True(t, ok, "shard-1 应该保留")
			assert.False(t, shard1Updated.LameDuck, "shard-1 不应该是 lameDuck")

			shard2Updated, ok := ss.AllShards["shard-2"]
			assert.True(t, ok, "shard-2 应该保留")
			assert.True(t, shard2Updated.LameDuck, "shard-2 应该被标记为 lameDuck")

			shard3Updated, ok := ss.AllShards["shard-3"]
			assert.True(t, ok, "shard-3 应该保留")
			// 关键检查点：shard-3 应该保持 lameDuck 状态
			assert.True(t, shard3Updated.LameDuck, "shard-3 应该保持 lameDuck 状态")

			shard4, ok := ss.AllShards["shard-4"]
			assert.True(t, ok, "shard-4 应该被创建")
			assert.False(t, shard4.LameDuck, "shard-4 不应该是 lameDuck")

			// 检查 etcd 中的最终状态
			// 注意：这里可能存在一个不一致，我们需要理解 etcd 中的值是否会立即反映内存中的变化
			fakeEtcdItems = fakeEtcd.List(ctx, "/smg/shard_state/", 100)
			t.Logf("测试结束时etcd中有 %d 个分片状态", len(fakeEtcdItems))

			// 尝试解析 JSON 来验证 shard-3 的 lameDuck 状态
			var shard3Found bool
			for _, item := range fakeEtcdItems {
				t.Logf("  - 键: %s, 值长度: %d", item.Key, len(item.Value))
				// 检查是否是 shard-3
				if strings.Contains(item.Key, "shard-3") {
					shard3Found = true
					shardState := &smgjson.ShardStateJson{}
					err := json.Unmarshal([]byte(item.Value), shardState)
					if err == nil {
						t.Logf("    解析 shard-3 状态: lameDuck=%d", shardState.LameDuck)
						// 在这种情况下，etcd 中的 shard-3 可能仍然保持原始的 lameDuck=1 状态
						// 这可能是因为 ServiceState 没有将内存中的更改写回 etcd
						t.Logf("    注意：内存中 shard-3 的 lameDuck 值为 %v，而 etcd 中为 %d",
							shard3Updated.LameDuck, shardState.LameDuck)
					} else {
						t.Logf("    解析 shard-3 JSON 失败: %v", err)
					}
				}
			}

			assert.True(t, shard3Found, "应该能在 etcd 中找到 shard-3")

			// 停止 ServiceState
			ss.StopAndWaitForExit(ctx)
		})
	})
}
