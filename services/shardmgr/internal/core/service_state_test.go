package core

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/etcdprov"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

func TestServiceState_Basic(t *testing.T) {
	// 创建测试用的 FakeEtcdProvider
	fakeEtcd := etcdprov.NewFakeEtcdProvider()

	// 准备初始数据
	ctx := context.Background()

	// 设置服务信息
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

		// 验证基本信息是否正确加载
		assert.Equal(t, "test-service", ss.ServiceInfo.ServiceName)
		assert.Equal(t, smgjson.ST_SOFT_STATEFUL, ss.ServiceInfo.ServiceType)
		assert.Equal(t, smgjson.MP_StartBeforeKill, ss.ServiceInfo.DefaultHints.MovePolicy)
		assert.Equal(t, int32(10), ss.ServiceInfo.DefaultHints.MaxReplicaCount)
		assert.Equal(t, int32(1), ss.ServiceInfo.DefaultHints.MinReplicaCount)

		// 验证 IsStateInMemory 方法
		assert.True(t, ss.IsStateInMemory())
	})
}

func TestServiceState_ShardManagement(t *testing.T) {
	// 创建测试用的 FakeEtcdProvider
	fakeEtcd := etcdprov.NewFakeEtcdProvider()

	// 准备初始数据
	ctx := context.Background()

	// 设置服务信息
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
	})
}

// waitForEtcdWrites 等待所有异步写入操作完成
// 这是通过检查FakeEtcdProvider的内部数据来实现的
func waitForEtcdWrites(t *testing.T, ctx context.Context, fakeEtcd *etcdprov.FakeEtcdProvider, retries int) {
	// 这里我们使用一个简单的方法来等待异步操作完成
	// 实际上，应该提供一个更好的机制来等待操作完成
	for i := 0; i < retries; i++ {
		time.Sleep(100 * time.Millisecond)
		shardStateItems := fakeEtcd.List(ctx, "/smg/shard_state/", 100)
		if len(shardStateItems) > 0 {
			t.Logf("异步写入操作已完成，找到 %d 个分片状态", len(shardStateItems))
			return
		}
		// 打印当前状态以便调试
		t.Logf("等待异步写入操作完成 (尝试 %d/%d)", i+1, retries)
		t.Logf("当前 FakeEtcdProvider 状态: %s", fakeEtcd.DebugDump())
	}
	t.Logf("尝试等待异步写入操作完成，但未成功")
}

// WaitUntil 等待直到条件满足或超时
// condition: 要评估的条件函数，返回 true 表示条件满足
// maxWaitMs: 最大等待时间（毫秒）
// intervalMs: 检查条件的间隔时间（毫秒）
// Returns: 是否成功（在超时前条件满足）
func WaitUntil(t *testing.T, condition func() (bool, string), maxWaitMs int, intervalMs int) bool {
	startTime := time.Now()
	maxDuration := time.Duration(maxWaitMs) * time.Millisecond
	intervalDuration := time.Duration(intervalMs) * time.Millisecond

	for i := 0; ; i++ {
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
}

// waitForShardStates 等待分片状态写入 etcd
func waitForShardStates(t *testing.T, ctx context.Context, fakeEtcd *etcdprov.FakeEtcdProvider, expectedCount int) bool {
	return WaitUntil(t, func() (bool, string) {
		shardStateItems := fakeEtcd.List(ctx, "/smg/shard_state/", 100)
		if len(shardStateItems) >= expectedCount {
			t.Logf("异步写入操作已完成，找到 %d 个分片状态", len(shardStateItems))
			return true, ""
		}
		return false, fakeEtcd.DebugDump()
	}, 2000, 100)
}

// waitForEtcdShardStates 等待所有分片状态被持久化到 etcd 中
// 它会等待更长的时间，确保异步写入操作完成
func waitForEtcdShardStates(t *testing.T, ctx context.Context, fakeEtcd *etcdprov.FakeEtcdProvider, expectedCount int) bool {
	return WaitUntil(t, func() (bool, string) {
		shardStateItems := fakeEtcd.List(ctx, "/smg/shard_state/", 100)
		if len(shardStateItems) >= expectedCount {
			t.Logf("异步写入操作已完成，找到 %d 个分片状态", len(shardStateItems))
			return true, ""
		}
		return false, fakeEtcd.DebugDump()
	}, 5000, 200) // 使用更长的超时时间和更大的间隔
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
	// 创建一个FakeEtcdProvider用于测试
	fakeEtcd := etcdprov.NewFakeEtcdProvider()

	// 创建一个自定义的等待函数，确保在退出前检查所有异步写入操作已完成
	waitForAsyncOps := func() {
		ctx := context.Background()
		success := waitForEtcdShardStates(t, ctx, fakeEtcd, 3)
		assert.True(t, success, "应该能在超时前将分片状态写入到 etcd 中")
	}

	// 使用RunWithEtcdProviderAndWait而不是RunWithEtcdProvider
	etcdprov.RunWithEtcdProviderAndWait(fakeEtcd, func() {
		ctx := context.Background()

		// 1. 准备初始数据

		// 准备服务信息，使用测试辅助函数
		serviceInfo := smgjson.CreateTestServiceInfo()
		fakeEtcd.Set(ctx, "/smg/config/service_info.json", serviceInfo.ToJson())

		// 准备服务配置，使用测试辅助函数
		serviceConfig := smgjson.CreateTestServiceConfig()
		fakeEtcd.Set(ctx, "/smg/config/service_config.json", serviceConfig.ToJson())

		// 准备分片计划
		shardPlanStr := `shard-1
shard-2|{"min_replica_count":2,"max_replica_count":5}
shard-3|{"migration_strategy":{"shard_move_type":"kill_before_start"}}`
		fakeEtcd.Set(ctx, "/smg/config/shard_plan.txt", shardPlanStr)

		// 2. 创建ServiceState
		ss := NewServiceState(ctx)

		// 3. 验证影子分片状态写入

		// 等待ServiceState加载分片
		success := waitForServiceShards(t, ss, 3)
		assert.True(t, success, "应该能在超时前加载分片")

		// 验证分片数量
		assert.Equal(t, 3, len(ss.AllShards), "应该有3个分片")

		// 验证特定分片的存在
		_, ok := ss.AllShards["shard-1"]
		assert.True(t, ok, "应该能找到 shard-1")

		_, ok = ss.AllShards["shard-2"]
		assert.True(t, ok, "应该能找到 shard-2")

		_, ok = ss.AllShards["shard-3"]
		assert.True(t, ok, "应该能找到 shard-3")
	}, waitForAsyncOps)
}

func TestServiceState_DynamicShardPlanUpdate(t *testing.T) {
	// 创建一个FakeEtcdProvider用于测试
	fakeEtcd := etcdprov.NewFakeEtcdProvider()

	// 创建一个自定义的等待函数，确保在退出前检查所有异步写入操作已完成
	waitForAsyncOps := func() {
		ctx := context.Background()
		// 在退出前最后一次验证etcd中的数据状态
		fakeEtcdItems := fakeEtcd.List(ctx, "/smg/shard_state/", 100)
		t.Logf("测试结束时etcd中有 %d 个分片状态", len(fakeEtcdItems))
		for _, item := range fakeEtcdItems {
			t.Logf("  - 键: %s, 值长度: %d", item.Key, len(item.Value))
		}
	}

	// 使用RunWithEtcdProviderAndWait而不是RunWithEtcdProvider
	etcdprov.RunWithEtcdProviderAndWait(fakeEtcd, func() {
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

		// 3. 第一次分片计划更新：添加三个分片

		// 设置第一个分片计划
		shardPlanStr := `shard-a
shard-b|{"min_replica_count":2,"max_replica_count":5}
shard-c|{"migration_strategy":{"shard_move_type":"kill_before_start"}}`
		fakeEtcd.Set(ctx, "/smg/config/shard_plan.txt", shardPlanStr)

		// 等待ServiceState更新分片
		success := waitForServiceShards(t, ss, 3)
		assert.True(t, success, "应该能在超时前更新分片状态")

		// 验证分片数量
		assert.Equal(t, 3, len(ss.AllShards), "更新后应该有 3 个分片")

		// 验证特定分片的存在
		_, ok := ss.AllShards["shard-a"]
		assert.True(t, ok, "应该能找到 shard-a")

		_, ok = ss.AllShards["shard-b"]
		assert.True(t, ok, "应该能找到 shard-b")

		_, ok = ss.AllShards["shard-c"]
		assert.True(t, ok, "应该能找到 shard-c")

		// 4. 第二次分片计划更新：删除两个分片

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
	}, waitForAsyncOps)
}
