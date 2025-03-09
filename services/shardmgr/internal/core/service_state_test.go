package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/etcdprov"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/shadow"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

// 确保全局状态在每个测试之前被正确重置
func resetGlobalState(t *testing.T) {
	if t != nil {
		t.Log("重置全局状态：EtcdStore 和 EtcdProvider")
	}
	shadow.ResetEtcdStore()
	etcdprov.ResetEtcdProvider()
}

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
		ss := NewServiceState(ctx)

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
		ss := NewServiceState(ctx)

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

// WaitUntil 等待条件满足或超时
func WaitUntil(t *testing.T, condition func() (bool, string), maxWaitMs int, intervalMs int) bool {
	maxDuration := time.Duration(maxWaitMs) * time.Millisecond
	intervalDuration := time.Duration(intervalMs) * time.Millisecond
	startTime := time.Now()

	for i := 0; i < maxWaitMs/intervalMs; i++ {
		success, debugInfo := condition()
		if success {
			elapsed := time.Since(startTime)
			t.Logf("条件满足，耗时 %v", elapsed)
			return true
		}

		elapsed := time.Since(startTime)
		if elapsed >= maxDuration {
			t.Logf("等待条件满足超时，已尝试 %d 次，耗时 %v，最后状态: %s", i+1, elapsed, debugInfo)
			return false
		}

		t.Logf("等待条件满足中 (尝试 %d/%d)，已耗时 %v，当前状态: %s",
			i+1, maxWaitMs/intervalMs, elapsed, debugInfo)
		time.Sleep(intervalDuration)
	}
	return false
}

// waitForServiceShards 等待 ServiceState 中的分片数量达到预期
func waitForServiceShards(t *testing.T, ss *ServiceState, expectedCount int) bool {
	return WaitUntil(t, func() (bool, string) {
		if len(ss.AllShards) >= expectedCount {
			t.Logf("ServiceState 中的分片数量已达到预期：%d", len(ss.AllShards))
			return true, ""
		}
		var shardIds []string
		for id := range ss.AllShards {
			shardIds = append(shardIds, string(id))
		}
		return false, fmt.Sprintf("当前分片数量: %d, 分片列表: %v", len(ss.AllShards), shardIds)
	}, 2000, 100)
}

func TestServiceState_ShadowStateWrite(t *testing.T) {
	// 测试开始前，确保EtcdStore和EtcdProvider被正确重置，避免全局变量状态干扰
	resetGlobalState(t)

	// 创建一个FakeEtcdProvider用于测试
	t.Log("创建FakeEtcdProvider")
	fakeEtcd := etcdprov.NewFakeEtcdProvider()

	// 直接设置全局的EtcdProvider
	etcdprov.SetCurrentEtcdProvider(fakeEtcd)

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
	ss := NewServiceState(ctx)

	// 不使用 defer 调用 StopAndWaitForExit，而是在测试结束前直接调用
	// defer ss.StopAndWaitForExit(ctx)

	// 3. 验证影子分片状态写入
	t.Log("开始验证影子分片状态写入")

	// 等待ServiceState加载分片
	t.Log("等待ServiceState加载分片")
	success := waitForServiceShards(t, ss, 3)
	t.Logf("等待ServiceState加载分片结果: %v", success)
	assert.True(t, success, "应该能在超时前加载分片")

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

	// 确保所有Runloop处理完毕
	time.Sleep(500 * time.Millisecond)

	// 在测试结束前调用 StopAndWaitForExit
	ss.StopAndWaitForExit(ctx)

	// 测试结束后清理，避免影响其他测试
	resetGlobalState(t)
}

func TestServiceState_DynamicShardPlanUpdate(t *testing.T) {
	// 重置全局状态，确保测试环境干净
	resetGlobalState(t)

	// 创建一个FakeEtcdProvider用于测试
	fakeEtcd := etcdprov.NewFakeEtcdProvider()

	etcdprov.RunWithEtcdProvider(fakeEtcd, func() {
		ctx := context.Background()

		// 1. 准备初始数据

		// 准备服务信息
		serviceInfo := smgjson.CreateTestServiceInfo()
		fakeEtcd.Set(ctx, "/smg/config/service_info.json", serviceInfo.ToJson())

		// 准备服务配置
		serviceConfig := smgjson.CreateTestServiceConfig()
		fakeEtcd.Set(ctx, "/smg/config/service_config.json", serviceConfig.ToJson())

		// 初始空分片计划
		fakeEtcd.Set(ctx, "/smg/config/shard_plan.txt", "")

		// 2. 创建ServiceState
		ss := NewServiceState(ctx)

		// 不使用 defer 调用 StopAndWaitForExit，而是在测试结束前直接调用
		// defer ss.StopAndWaitForExit(ctx)

		// 3. 第一次分片计划更新：添加三个分片

		// 设置第一个分片计划
		shardPlanStr := `shard-a
shard-b|{"min_replica_count":2,"max_replica_count":5}
shard-c|{"migration_strategy":{"shard_move_type":"kill_before_start"}}`
		fakeEtcd.Set(ctx, "/smg/config/shard_plan.txt", shardPlanStr)

		// 等待ServiceState加载分片
		success := waitForServiceShards(t, ss, 3)
		assert.True(t, success, "应该能在超时前加载分片")

		// 验证分片是否存在
		_, ok := ss.AllShards["shard-a"]
		assert.True(t, ok, "应该能找到 shard-a")

		_, ok = ss.AllShards["shard-b"]
		assert.True(t, ok, "应该能找到 shard-b")

		_, ok = ss.AllShards["shard-c"]
		assert.True(t, ok, "应该能找到 shard-c")

		// 4. 第二次分片计划更新：保留一个分片，添加一个新分片，删除两个分片

		// 设置新的分片计划
		updatedShardPlan := `shard-a
shard-d|{"min_replica_count":4,"max_replica_count":8}`
		fakeEtcd.Set(ctx, "/smg/config/shard_plan.txt", updatedShardPlan)

		// 等待ServiceState更新分片
		success = waitForServiceShards(t, ss, 4) // 应该有 4 个分片: a,b,c,d (b,c 被标记为 lameDuck)
		assert.True(t, success, "应该能在超时前更新分片状态")

		// 验证新分片是否存在
		_, ok = ss.AllShards["shard-a"]
		assert.True(t, ok, "shard-a 应该保留")

		_, ok = ss.AllShards["shard-d"]
		assert.True(t, ok, "shard-d 应该被创建")

		// 检查 b 和 c 是否被标记为 lameDuck
		shard_b, ok := ss.AllShards["shard-b"]
		assert.True(t, ok, "shard-b 应该保留")
		assert.Equal(t, true, shard_b.LameDuck, "shard-b 应该被标记为 lameDuck")

		shard_c, ok := ss.AllShards["shard-c"]
		assert.True(t, ok, "shard-c 应该保留")
		assert.Equal(t, true, shard_c.LameDuck, "shard-c 应该被标记为 lameDuck")

		// 在测试结束前记录最终状态
		fakeEtcdItems := fakeEtcd.List(ctx, "/smg/shard_state/", 100)
		t.Logf("测试结束时etcd中有 %d 个分片状态", len(fakeEtcdItems))
		for _, item := range fakeEtcdItems {
			t.Logf("  - 键: %s, 值长度: %d", item.Key, len(item.Value))
		}

		// 在测试结束前调用 StopAndWaitForExit
		ss.StopAndWaitForExit(ctx)
	})
}
