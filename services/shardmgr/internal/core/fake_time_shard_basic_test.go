package core

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/etcdprov"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

// FakeTime style test case
// 1. **测试目的**：验证系统如何处理分片计划动态变更

// 2. **初始设置**：
//    - 创建空的分片计划
//    - 设置基本服务配置

// 3. **第一次变更**：
//    - 添加三个分片：shard-a、shard-b、shard-c
//    - 验证系统正确创建了这些分片

// 4. **第二次变更**：
//    - 保留shard-a，添加新分片shard-d
//    - 移除shard-b和shard-c
//    - 验证移除的分片被标记为lameDuck（准备下线）

// 5. **验证点**：
//    - 系统能正确添加和识别新分片
//    - 移除的分片不会立即删除而是标记为lameDuck
//    - 所有状态变更被正确持久化

func TestServiceState_DynamicShardPlanUpdate(t *testing.T) {
	ctx := context.Background()
	klogging.SetDefaultLogger(klogging.NewLogrusLogger(ctx).SetConfig(ctx, "debug", "text"))
	klogging.Info(ctx).Log("DynamicShardPlanUpdate", "设置测试环境...")

	// 配置测试环境
	setup := NewFakeTimeTestSetup(t)
	setup.SetupBasicConfig(ctx)
	klogging.Info(ctx).Log("DynamicShardPlanUpdate", "测试环境已配置")

	// 使用FakeTime环境运行测试
	setup.RunWith(func() {
		// 创建 ServiceState (shard plan为空)
		ss := AssembleSsWithShadowState(ctx, "TestServiceState_DynamicShardPlanUpdate")
		setup.ServiceState = ss
		klogging.Info(ctx).With("serviceName", ss.Name).Log("DynamicShardPlanUpdate", "ServiceState已创建")

		// 初始阶段：验证服务状态的初始状态 - 应该没有分片
		klogging.Info(ctx).Log("DynamicShardPlanUpdate", "验证初始状态...")
		{
			var initialCount int
			safeAccessServiceState(ss, func(ss *ServiceState) {
				initialCount = len(ss.AllShards)
			})
			klogging.Info(ctx).With("initialCount", initialCount).Log("DynamicShardPlanUpdate", "初始状态")
		}

		// 第一次更新：添加三个分片 (shard-a, shard-b, shard-c)
		klogging.Info(ctx).Log("DynamicShardPlanUpdate", "第一次更新：添加三个分片 (shard-a, shard-b, shard-c)")
		firstShardPlan := []string{"shard-a", "shard-b", "shard-c"}
		setShardPlan(t, setup.FakeEtcd, ctx, firstShardPlan)

		// 等待ServiceState加载分片状态
		klogging.Info(ctx).Log("DynamicShardPlanUpdate", "等待ServiceState加载分片状态...")
		{
			waitSucc, elapsedMs := setup.WaitUntilShardCount(t, 3, 1000, 100)
			assert.True(t, waitSucc, "应该能在超时前加载所有分片, 耗时=%dms", elapsedMs)
			klogging.Info(ctx).With("elapsedMs", elapsedMs).Log("DynamicShardPlanUpdate", "分片加载完成")
		}

		// 验证第一次更新后的分片状态
		klogging.Info(ctx).Log("DynamicShardPlanUpdate", "验证第一次更新后的分片状态...")
		expectedStates1 := map[data.ShardId]ExpectedShardState{
			"shard-a": {LameDuck: false},
			"shard-b": {LameDuck: false},
			"shard-c": {LameDuck: false},
		}
		errors1 := verifyAllShardState(t, ss, expectedStates1)
		assert.Empty(t, errors1, "第一次更新后分片状态验证失败: %v", errors1)

		// 确保状态已持久化
		klogging.Info(ctx).Log("DynamicShardPlanUpdate", "确保第一次更新的状态已持久化...")
		setup.FakeTime.VirtualTimeForward(ctx, 500) // 前进500ms，确保持久化完成
		errors1storage := verifyAllShardsInStorage(t, setup, expectedStates1)
		assert.Empty(t, errors1storage, "第一次更新后分片状态持久化验证失败: %v", errors1storage)

		// 第二次更新：保留shard-a，添加shard-d，移除shard-b和shard-c
		klogging.Info(ctx).Log("DynamicShardPlanUpdate", "第二次更新：保留shard-a，添加shard-d，移除shard-b和shard-c")
		secondShardPlan := []string{"shard-a", "shard-d"}
		setShardPlan(t, setup.FakeEtcd, ctx, secondShardPlan)

		// 等待ServiceState更新分片状态（保留原有分片并标记删除）
		// 预期会有4个分片：shard-a（保留）, shard-b（标记删除）, shard-c（标记删除）, shard-d（新增）
		klogging.Info(ctx).Log("DynamicShardPlanUpdate", "等待ServiceState更新分片状态...")
		{
			waitSucc, elapsedMs := setup.WaitUntilShardCount(t, 4, 1000, 100)
			assert.True(t, waitSucc, "应该能在超时前更新所有分片, 耗时=%dms", elapsedMs)
			klogging.Info(ctx).With("elapsedMs", elapsedMs).Log("DynamicShardPlanUpdate", "分片更新完成")
		}

		// 验证第二次更新后的分片状态
		klogging.Info(ctx).Log("DynamicShardPlanUpdate", "验证第二次更新后的分片状态...")
		expectedStates2 := map[data.ShardId]ExpectedShardState{
			"shard-a": {LameDuck: false}, // 保留的分片
			"shard-b": {LameDuck: true},  // 被移除的分片，标记为lameDuck
			"shard-c": {LameDuck: true},  // 被移除的分片，标记为lameDuck
			"shard-d": {LameDuck: false}, // 新增的分片
		}
		errors2 := verifyAllShardState(t, ss, expectedStates2)
		assert.Empty(t, errors2, "第二次更新后分片状态验证失败: %v", errors2)

		// 确保状态已持久化
		klogging.Info(ctx).Log("DynamicShardPlanUpdate", "确保第二次更新的状态已持久化...")
		setup.FakeTime.VirtualTimeForward(ctx, 500) // 前进500ms，确保持久化完成
		errors2storage := verifyAllShardsInStorage(t, setup, expectedStates2)
		assert.Empty(t, errors2storage, "第二次更新后分片状态持久化验证失败: %v", errors2storage)

		// 输出最终状态以供参考
		safeAccessServiceState(ss, func(ss *ServiceState) {
			klogging.Info(ctx).With("shardCount", len(ss.AllShards)).Log("DynamicShardPlanUpdate", "最终状态")
			for shardId, shard := range ss.AllShards {
				klogging.Info(ctx).With("shardId", shardId).With("lameDuck", shard.LameDuck).Log("DynamicShardPlanUpdate", "分片状态")
			}
		})

		// 停止ServiceState
		ss.StopAndWaitForExit(ctx)
		klogging.Info(ctx).Log("DynamicShardPlanUpdate", "测试完成，ServiceState已停止")
	})
}

// FakeTime style test case
// **测试场景**：验证分片状态的持久化流程
//
// **初始状态**：
// - 系统配置了包含3个分片的分片计划
// - 尚未创建任何实际分片状态
//
// **触发行为**：
// - 启动服务状态管理器（加载配置并初始化系统）
// - 系统根据分片计划创建分片状态
//
// **期望结果**：
// - 所有分片被正确加载到内存中
// - 分片状态被自动持久化到存储系统
// - 存储系统中包含与内存状态一致的完整分片信息
func TestServiceState_ShadowStateWrite(t *testing.T) {
	ctx := context.Background()
	klogging.SetDefaultLogger(klogging.NewLogrusLogger(ctx).SetConfig(ctx, "debug", "text"))
	t.Logf("设置测试环境...")

	// 配置测试环境
	setup := NewFakeTimeTestSetup(t)
	setup.SetupBasicConfig(ctx)
	t.Logf("测试环境已配置")

	// 使用FakeTime环境运行测试
	setup.RunWith(func() {
		// 1. 准备分片计划
		t.Logf("准备分片计划...")
		shardPlan := []string{
			"shard-a",
			"shard-b|{\"min_replica_count\":2,\"max_replica_count\":5}",
			"shard-c|{\"move_type\":\"kill_before_start\"}",
		}
		setShardPlan(t, setup.FakeEtcd, ctx, shardPlan)
		t.Logf("分片计划已设置：%v", shardPlan)

		// 记录初始写入次数，用于后续验证
		initialPutCount := setup.FakeStore.GetPutCalledCount()
		t.Logf("初始EtcdStore写入次数: %d", initialPutCount)

		// 2. 创建 ServiceState
		ss := AssembleSsWithShadowState(ctx, "TestServiceState_ShadowStateWrite")
		setup.ServiceState = ss
		t.Logf("ServiceState已创建: %s", ss.Name)

		// 3. 等待ServiceState加载分片状态
		t.Logf("等待ServiceState加载分片状态...")
		{
			waitSucc, elapsedMs := setup.WaitUntilShardCount(t, 3, 1000, 100)
			assert.True(t, waitSucc, "应该能在超时前加载所有分片, 耗时=%dms", elapsedMs)
			t.Logf("分片加载完成, 耗时=%dms", elapsedMs)
		}

		// 4. 验证分片内存状态
		t.Logf("验证分片内存状态...")
		// 使用safeAccessServiceState安全访问ServiceState内部状态
		safeAccessServiceState(ss, func(ss *ServiceState) {
			// 验证分片数量
			assert.Equal(t, 3, len(ss.AllShards), "内存中应有3个分片")

			// 验证特定分片存在
			_, ok := ss.AllShards["shard-a"]
			assert.True(t, ok, "应能找到shard-a")

			_, ok = ss.AllShards["shard-b"]
			assert.True(t, ok, "应能找到shard-b")

			_, ok = ss.AllShards["shard-c"]
			assert.True(t, ok, "应能找到shard-c")

			t.Logf("内存中的分片状态验证通过：共%d个分片", len(ss.AllShards))
		})

		// 5. 前进虚拟时间，以确保持久化有足够时间完成
		t.Logf("推进虚拟时间以确保状态持久化...")
		setup.FakeTime.VirtualTimeForward(ctx, 500) // 前进500ms

		// 6. 验证分片状态持久化
		t.Logf("验证分片状态持久化...")
		// 验证写入次数增加
		currentPutCount := setup.FakeStore.GetPutCalledCount()
		t.Logf("当前EtcdStore写入次数: %d, 增加了: %d", currentPutCount, currentPutCount-initialPutCount)
		assert.Greater(t, currentPutCount, initialPutCount, "应该有新的写入操作")

		// 验证存储中的数据
		storeItems := setup.FakeStore.List("/smg/shard_state/")
		t.Logf("存储中的分片状态数量: %d", len(storeItems))
		assert.GreaterOrEqual(t, len(storeItems), 3, "存储中应至少有3个分片状态")

		for _, item := range storeItems {
			t.Logf("  - 存储项: %s, 值长度: %d", item.Key, len(item.Value))
		}

		// 7. 验证存储内容与内存状态一致
		t.Logf("验证存储内容与内存状态一致...")
		expectedShardStates := map[data.ShardId]ExpectedShardState{
			"shard-a": {LameDuck: false},
			"shard-b": {LameDuck: false},
			"shard-c": {LameDuck: false},
		}
		errors := verifyAllShardsInStorage(t, setup, expectedShardStates)
		assert.Empty(t, errors, "分片状态持久化验证失败: %v", errors)
		t.Logf("分片状态持久化验证通过")

		// 8. 停止ServiceState
		ss.StopAndWaitForExit(ctx)
		t.Logf("测试完成，ServiceState已停止")
	})
}

// FakeTime style test case
// ## 测试目的
// 验证系统能否正确处理已存在于存储中的分片状态，特别是在分片计划变更时的行为。

// ## 测试流程
// 1. **初始设置**：
//    - 创建分片计划（shard-1, shard-2）
//    - 预先在存储中创建三个分片状态：
//      * shard-1：正常状态
//      * shard-2：正常状态
//      * shard-3：lameDuck状态（不在计划中）

// 2. **启动服务**：
//    - 创建ServiceState实例
//    - 验证三个分片状态被正确加载

// 3. **计划变更**：
//    - 更新分片计划（保留shard-1，添加shard-4）
//    - 验证状态变化：
//      * shard-1：保持正常
//      * shard-2：变为lameDuck（已移除）
//      * shard-3：保持lameDuck（不在计划中）
//      * shard-4：新创建为正常状态

// ## 关键验证点
// - 系统能正确加载已存在的分片状态
// - 分片状态会根据计划变更自动调整（添加/移除/保持）
// - 状态变更应正确持久化到存储系统

// TestServiceState_PreexistingShardState 测试预先存在的分片状态如何被加载和更新
func TestServiceState_PreexistingShardState(t *testing.T) {
	ctx := context.Background()
	klogging.SetDefaultLogger(klogging.NewLogrusLogger(ctx).SetConfig(ctx, "debug", "text"))

	// 配置测试环境
	setup := NewFakeTimeTestSetup(t)
	setup.SetupBasicConfig(ctx)
	t.Logf("测试环境已配置")

	// 使用辅助函数运行测试，确保资源正确清理
	setup.RunWith(func() {
		// 1. 设置初始分片计划
		initialShardPlan := []string{"shard-1", "shard-2", "shard-3"}
		setShardPlan(t, setup.FakeEtcd, ctx, initialShardPlan)
		t.Logf("初始分片计划已设置：%v", initialShardPlan)

		// 2. 在 etcd 中预先创建分片状态，包括一些不在计划中的分片和 lameDuck 标记
		// 创建几种不同情况的分片状态:
		// - shard-1: 在计划中，正常状态（非 lameDuck）
		// - shard-2: 在计划中，但标记为 lameDuck（异常状态）
		// - shard-3: 在计划中，正常状态
		// - shard-4: 不在计划中，标记为 lameDuck（正常）
		// - shard-5: 不在计划中，但没有标记 lameDuck（异常状态）
		preExistingShardStates := map[data.ShardId]smgjson.ShardStateJson{
			"shard-1": {ShardName: "shard-1", LameDuck: 0}, // 正常状态
			"shard-2": {ShardName: "shard-2", LameDuck: 1}, // 异常状态（应该被自动修复）
			"shard-3": {ShardName: "shard-3", LameDuck: 0}, // 正常状态
			"shard-4": {ShardName: "shard-4", LameDuck: 1}, // 正常状态（不在计划中）
			"shard-5": {ShardName: "shard-5", LameDuck: 0}, // 异常状态（不在计划中，应该被自动修复）
		}

		// 将预先存在的分片状态写入 etcd
		for shardId, shardState := range preExistingShardStates {
			shardStatePath := fmt.Sprintf("/smg/shard_state/%s", shardId)
			setup.FakeEtcd.Set(ctx, shardStatePath, shardState.ToJson())
			t.Logf("预设分片状态: %s, lameDuck=%v", shardId, shardState.LameDuck == 1)
		}

		// 3. 创建 ServiceState 实例，它会加载预先存在的分片状态
		ss := AssembleSsWithShadowState(ctx, "TestServiceState_PreexistingShardState")
		setup.ServiceState = ss
		t.Logf("ServiceState 已创建: %s", ss.Name)

		// 4. 等待 ServiceState 加载和处理分片状态
		waitSucc, elapsedMs := setup.WaitUntilShardCount(t, 3, 1000, 100)
		assert.True(t, waitSucc, "应该能在超时前加载所有预先存在的分片状态, 耗时=%dms", elapsedMs)
		t.Logf("分片加载完成, 耗时=%dms", elapsedMs)

		// 5. 验证分片状态是否被正确加载和修复
		// 预期状态（ServiceState 应该自动修复异常情况）:
		// - shard-1: 在计划中，正常状态（非 lameDuck）
		// - shard-2: 在计划中，应该被修复为非 lameDuck
		// - shard-3: 在计划中，正常状态（非 lameDuck）
		// - shard-4: 不在计划中，保持 lameDuck 状态
		// - shard-5: 不在计划中，应该被修复为 lameDuck
		expectedShardStates := map[data.ShardId]ExpectedShardState{
			"shard-1": {LameDuck: false}, // 在计划中，应为正常状态
			"shard-2": {LameDuck: false}, // 在计划中，应被修复为正常状态
			"shard-3": {LameDuck: false}, // 在计划中，应为正常状态
			// "shard-4": {LameDuck: true},  // 不在计划中，应为 lameDuck 状态
			// "shard-5": {LameDuck: true},  // 不在计划中，应被修复为 lameDuck 状态
		}

		allErrors := verifyAllShardState(t, ss, expectedShardStates)
		assert.Equal(t, 0, len(allErrors), "分片状态验证应该全部通过，错误: %v", allErrors)
		t.Logf("分片状态验证完成: 所有分片状态符合预期")

		// 6. 验证分片状态是否被正确持久化到 etcd
		{
			succ, persistenceErrors := WaitUntil(t, func() (bool, string) {
				// 验证所有分片状态是否正确持久化
				persistenceErrors := verifyAllShardsInStorage(t, setup, expectedShardStates)
				if len(persistenceErrors) > 0 {
					return false, strings.Join(persistenceErrors, "\n")
				}
				return true, ""
			}, 1000, 100)
			// persistenceErrors := verifyAllShardsInStorage(t, setup, expectedShardStates)
			assert.Equal(t, true, succ, "分片状态持久化验证应该全部通过，错误: %v", persistenceErrors)
			t.Logf("分片状态持久化验证完成: 所有分片状态已正确持久化")
		}

		// 7. 更新分片计划，测试动态更新能力
		updatedShardPlan := []string{"shard-1", "shard-3", "shard-5", "shard-6"}
		setShardPlan(t, setup.FakeEtcd, ctx, updatedShardPlan)
		t.Logf("分片计划已更新：%v", updatedShardPlan)

		// 8. 等待更新后的状态生效
		// 分片总数应该是 6: shard-1, shard-2(lameDuck), shard-3, shard-4(lameDuck), shard-5(不再lameDuck), shard-6(新增)
		waitSucc, elapsedMs = setup.WaitUntilShardCount(t, 5, 1000, 100)
		assert.True(t, waitSucc, "应该能在超时前更新分片状态, 耗时=%dms", elapsedMs)
		t.Logf("分片状态更新完成, 耗时=%dms", elapsedMs)

		// 9. 验证更新后的分片状态
		updatedExpectedStates := map[data.ShardId]ExpectedShardState{
			"shard-1": {LameDuck: false}, // 仍在计划中，保持正常状态
			// "shard-2": {LameDuck: true},  // 不再在计划中，应变为 lameDuck
			"shard-3": {LameDuck: false}, // 仍在计划中，保持正常状态
			// "shard-4": {LameDuck: true},  // 仍不在计划中，保持 lameDuck
			"shard-5": {LameDuck: false}, // 新加入计划，应变为正常状态
			"shard-6": {LameDuck: false}, // 新加入计划，应为正常状态
		}

		{
			allErrors = verifyAllShardState(t, ss, updatedExpectedStates)
			assert.Equal(t, 0, len(allErrors), "更新后的分片状态验证应该全部通过，错误: %v", allErrors)
			t.Logf("更新后的分片状态验证完成: 所有分片状态符合预期")
		}

		// 10. 验证更新后的分片状态是否被正确持久化
		{
			succ, elapsedMs := WaitUntil(t, func() (bool, string) {
				// 验证所有分片状态是否正确持久化
				result := verifyAllShardsInStorage(t, setup, updatedExpectedStates)
				if len(result) > 0 {
					return false, strings.Join(result, "\n")
				}
				return true, ""
			}, 100, 10)
			// persistenceErrors := verifyAllShardsInStorage(t, setup, updatedExpectedStates)
			assert.Equal(t, true, succ, "更新后的分片状态持久化验证应该全部通过, 耗时=%dms", elapsedMs)
			t.Logf("更新后的分片状态持久化验证完成: 所有分片状态已正确持久化")
		}

		// 结束前停止 ServiceState
		ss.StopAndWaitForExit(ctx)
		t.Logf("ServiceState已停止，测试完成")
	})
}

// ### 1. 一致性验证（ConsistencyCheck）
// **测试场景**：当初始分片状态与分片计划完全一致时的系统行为。
// **具体步骤**：
// 1. 设置初始分片计划，包含3个分片：`shard-1`、`shard-2`和`shard-3`
// 2. 创建与分片计划完全一致的分片状态，所有分片都处于非lameDuck状态
// 3. 创建并初始化`ServiceState`
// 4. 等待加载预先存在的分片状态
// 5. 验证所有分片状态都正确加载，且都是非lameDuck状态
// **期望结果**：系统应该正确加载所有分片，并保持它们的原始状态（非lameDuck）。

func TestShardBasic_ConsistencyCheck(t *testing.T) {
	ctx := context.Background()
	klogging.SetDefaultLogger(klogging.NewLogrusLogger(ctx).SetConfig(ctx, "debug", "text"))

	// 配置测试环境
	setup := NewFakeTimeTestSetup(t)
	setup.SetupBasicConfig(ctx)
	// fakeTime := setup.FakeTime
	t.Logf("测试环境已配置")

	fn := func() {
		// 创建 ServiceState
		ss := AssembleSsWithShadowState(ctx, "TestShardBasic_ConsistencyCheck")
		setup.ServiceState = ss
		t.Logf("ServiceState已创建: %s", ss.Name)

		// 1. 设置初始分片计划，包含3个分片
		shardPlan := []string{"shard-1", "shard-2", "shard-3"}
		setShardPlan(t, setup.FakeEtcd, ctx, shardPlan)
		t.Logf("分片计划已设置：%v", shardPlan)

		// 3. 等待ServiceState加载分片状态
		t.Logf("等待ServiceState加载分片状态...")
		// 验证所有分片状态都正确加载
		{
			waitSucc, elapsedMs := setup.WaitUntilShardCount(t, 3, 1000, 100)
			assert.True(t, waitSucc, "应该能在超时前加载所有分片, 耗时=%dms", elapsedMs)
			t.Logf("分片加载完成, 耗时=%dms", elapsedMs)
		}

		expectedShardStates := map[data.ShardId]ExpectedShardState{
			"shard-1": {LameDuck: false},
			"shard-2": {LameDuck: false},
			"shard-3": {LameDuck: false},
		}
		{
			allPassed := verifyAllShardState(t, ss, expectedShardStates)
			assert.Equal(t, 0, len(allPassed), "分片状态验证应该全部通过, result=%v", allPassed)
			t.Logf("分片状态验证完成: 所有分片状态符合预期")
		}

		// 5. 等待分片状态持久化
		{
			waitSucc, elapsedMs := WaitUntil(t, func() (bool, string) {
				results := verifyAllShardsInStorage(t, setup, expectedShardStates)
				if len(results) > 0 {
					return false, strings.Join(results, ",")
				}
				return true, ""
			}, 100, 10)
			assert.True(t, waitSucc, "分片状态持久化验证应该全部通过, 耗时=%dms", elapsedMs)
		}

		// 6. 验证分片状态持久化
		{
			results := verifyAllShardsInStorage(t, setup, expectedShardStates)
			assert.Equal(t, 0, len(results), "分片状态持久化验证应该全部通过, result=%v", results)
		}

		// 结束前停止 ServiceState
		ss.StopAndWaitForExit(ctx)
		t.Logf("ServiceState已停止，测试完成")
	}

	// 使用 FakeTimeProvider 和模拟的 EtcdProvider/EtcdStore 运行测试
	setup.RunWith(fn)
}

// ### 2. 冲突解决（ConflictResolution）

// **测试场景**：当部分分片在etcd中被标记为lameDuck，但在分片计划中存在时的系统行为。

// **具体步骤**：
// 1. 设置初始分片计划，包含3个分片：`shard-1`、`shard-2`和`shard-3`
// 2. 创建与分片计划有冲突的分片状态：
//    - `shard-1`：正常状态（非lameDuck）
//    - `shard-2`：lameDuck状态（但在分片计划中存在）
//    - `shard-3`：正常状态（非lameDuck）
// 3. 创建并初始化`ServiceState`
// 4. 等待加载预先存在的分片状态
// 5. 验证分片状态是否被正确调整，特别是`shard-2`是否从lameDuck状态调整为非lameDuck状态

// **期望结果**：系统应该根据分片计划自动修复冲突，将`shard-2`的状态从lameDuck调整为非lameDuck。
func TestShardBasic_ConflictResolution(t *testing.T) {
	ctx := context.Background()
	klogging.SetDefaultLogger(klogging.NewLogrusLogger(ctx).SetConfig(ctx, "debug", "text"))

	// 配置测试环境
	setup := NewFakeTimeTestSetup(t)
	setup.SetupBasicConfig(ctx)
	t.Logf("测试环境已配置")

	fn := func() {
		// 创建 ServiceState
		ss := AssembleSsWithShadowState(ctx, "TestShardBasic_ConflictResolution")
		setup.ServiceState = ss
		t.Logf("ServiceState已创建: %s", ss.Name)

		// 1. 设置初始分片计划，包含3个分片
		shardPlan := []string{"shard-1", "shard-2", "shard-3"}
		setShardPlan(t, setup.FakeEtcd, ctx, shardPlan)
		t.Logf("分片计划已设置：%v", shardPlan)

		// 2. 在etcd中创建与分片计划有冲突的分片状态
		// 创建初始分片状态：shard-1和shard-3为正常状态，shard-2为lameDuck状态
		pm := ss.PathManager
		shardStates := map[string]bool{
			"shard-1": false, // 正常状态
			"shard-2": true,  // lameDuck状态（但在分片计划中存在）
			"shard-3": false, // 正常状态
		}

		// 在etcd中创建初始分片状态
		for shardName, isLameDuck := range shardStates {
			shardId := data.ShardId(shardName)
			// 使用smgjson.ShardStateJson来创建分片状态
			shardState := &smgjson.ShardStateJson{
				ShardName: shardId,
				LameDuck:  0,
			}
			if isLameDuck {
				shardState.LameDuck = 1
				t.Logf("将分片 %s 设置为lameDuck状态", shardName)
			}

			shardStatePath := pm.FmtShardStatePath(shardId)
			setup.FakeEtcd.Set(ctx, shardStatePath, shardState.ToJson())
			t.Logf("已创建分片状态: %s, lameDuck=%v, 路径=%s", shardName, isLameDuck, shardStatePath)
		}

		// 3. 等待ServiceState加载分片状态
		t.Logf("等待ServiceState加载分片状态...")
		{
			waitSucc, elapsedMs := setup.WaitUntilShardCount(t, 3, 1000, 100)
			assert.True(t, waitSucc, "应该能在超时前加载所有分片, 耗时=%dms", elapsedMs)
			t.Logf("分片加载完成, 耗时=%dms", elapsedMs)
		}

		// 4. 验证分片状态是否被正确调整
		// 期望的状态：所有分片都应该是非lameDuck状态，因为它们都在分片计划中
		expectedShardStates := map[data.ShardId]ExpectedShardState{
			"shard-1": {LameDuck: false},
			"shard-2": {LameDuck: false}, // 虽然初始是lameDuck，但应被系统自动调整为非lameDuck
			"shard-3": {LameDuck: false},
		}

		// 验证所有分片状态是否正确
		{
			allPassed := verifyAllShardState(t, ss, expectedShardStates)
			assert.Equal(t, 0, len(allPassed), "分片状态验证应该全部通过（所有分片都应为非lameDuck）, result=%v", allPassed)
			t.Logf("分片状态验证完成: 所有分片状态符合预期")

			// 特别验证shard-2是否被调整为非lameDuck
			var shard2State *ShardState
			var found bool
			safeAccessServiceState(ss, func(ss *ServiceState) {
				shard2State, found = ss.AllShards["shard-2"]
			})
			assert.True(t, found, "应能找到shard-2")
			if found {
				assert.False(t, shard2State.LameDuck, "shard-2应被调整为非lameDuck状态")
				t.Logf("验证成功：shard-2被正确调整为非lameDuck状态")
			}
		}

		// 5. 验证分片状态持久化
		{
			waitSucc, elapsedMs := WaitUntil(t, func() (bool, string) {
				results := verifyAllShardsInStorage(t, setup, expectedShardStates)
				if len(results) > 0 {
					return false, strings.Join(results, ",")
				}
				return true, ""
			}, 100, 10)
			assert.True(t, waitSucc, "分片状态持久化验证应该全部通过, 耗时=%dms", elapsedMs)
			t.Logf("分片状态持久化验证完成，耗时=%dms", elapsedMs)
		}

		// 再次明确验证shard-2在存储中也是非lameDuck状态
		{
			shardId := data.ShardId("shard-2")
			shardStatePath := pm.FmtShardStatePath(shardId)
			val := setup.FakeStore.GetByKey(shardStatePath)
			assert.NotEqual(t, "", val, "应能找到shard-2的持久化状态")

			if val != "" {
				shardState := smgjson.ShardStateJsonFromJson(val)
				assert.Equal(t, shardId, shardState.ShardName, "持久化的ShardName应正确")
				assert.Equal(t, int8(0), shardState.LameDuck, "持久化的shard-2应调整为非lameDuck状态")
				t.Logf("持久化验证成功：存储中的shard-2状态已被调整为非lameDuck（LameDuck=%d）", shardState.LameDuck)
			}
		}

		// 结束前停止 ServiceState
		ss.StopAndWaitForExit(ctx)
		t.Logf("ServiceState已停止，测试完成")
	}

	// 使用 FakeTimeProvider 和模拟的 EtcdProvider/EtcdStore 运行测试
	setup.RunWith(fn)
}

// ### 3. 动态计划更新（DynamicPlanUpdate）

// **测试场景**：当分片计划动态变化时，系统对现有分片状态的调整行为。

// **具体步骤**：
// 1. 设置初始分片计划，包含3个分片：`shard-1`、`shard-2`和`shard-3`
// 2. 创建初始分片状态：
//    - `shard-1`、`shard-2`、`shard-3`：正常状态（非lameDuck）
//    - `shard-4`：lameDuck状态（不在初始计划中）
// 3. 创建并初始化`ServiceState`
// 4. 验证初始状态正确：计划内的分片为非lameDuck，计划外的`shard-4`为lameDuck
// 5. 更新分片计划：移除`shard-2`，添加`shard-4`（新计划：`shard-1`、`shard-3`、`shard-4`）
// 6. 等待分片状态更新
// 7. 验证分片状态是否根据更新后的计划正确调整：
//    - `shard-2`应变为lameDuck（因为已从计划中移除）
//    - `shard-4`应变为非lameDuck（因为已加入计划）

// **期望结果**：系统应该根据更新后的分片计划自动调整分片状态，将被移出计划的分片标记为lameDuck，将新加入计划的分片从lameDuck恢复为正常状态。

func TestShardBasic_DynamicPlanUpdate(t *testing.T) {
	ctx := context.Background()
	klogging.SetDefaultLogger(klogging.NewLogrusLogger(ctx).SetConfig(ctx, "debug", "text"))

	// 配置测试环境
	setup := NewFakeTimeTestSetup(t)
	setup.SetupBasicConfig(ctx)
	t.Logf("测试环境已配置")

	fn := func() {
		// 创建 ServiceState
		ss := AssembleSsWithShadowState(ctx, "TestShardBasic_DynamicPlanUpdate")
		setup.ServiceState = ss
		t.Logf("ServiceState已创建: %s", ss.Name)

		// 1. 设置初始分片计划，包含3个分片
		initialShardPlan := []string{"shard-1", "shard-2", "shard-3"}
		setShardPlan(t, setup.FakeEtcd, ctx, initialShardPlan)
		t.Logf("初始分片计划已设置：%v", initialShardPlan)

		// 2. 等待ServiceState加载分片状态
		t.Logf("等待ServiceState加载初始分片状态...")
		{
			waitSucc, elapsedMs := setup.WaitUntilShardCount(t, 3, 1000, 100)
			assert.True(t, waitSucc, "应该能在超时前加载所有分片, 耗时=%dms", elapsedMs)
			t.Logf("分片加载完成, 耗时=%dms", elapsedMs)
		}

		{
			newShardPlan := []string{"shard-1", "shard-2", "shard-3", "shard-4"} // add shard-4
			t.Logf("更新分片计划: %v -> %v", initialShardPlan, newShardPlan)
			setShardPlan(t, setup.FakeEtcd, ctx, newShardPlan)
		}

		// 3. 等待ServiceState shard-4状态更新
		{
			waitSucc, elapsedMs := setup.WaitUntilShardCount(t, 4, 1000, 100)
			assert.True(t, waitSucc, "应该能在超时前加载所有分片, 耗时=%dms", elapsedMs)
			t.Logf("分片加载完成, 耗时=%dms", elapsedMs)
		}

		// 4. 验证初始分片状态

		// 验证所有分片的初始状态
		{
			// 期望的初始状态：计划内分片为非lameDuck，计划外分片为lameDuck
			initialExpectedStates := map[data.ShardId]ExpectedShardState{
				"shard-1": {LameDuck: false}, // 在计划内
				"shard-2": {LameDuck: false}, // 在计划内
				"shard-3": {LameDuck: false}, // 在计划内
				"shard-4": {LameDuck: false}, // 在计划内
			}
			allPassed := verifyAllShardState(t, ss, initialExpectedStates)
			assert.Equal(t, 0, len(allPassed), "初始分片状态验证应该全部通过, result=%v", allPassed)
			t.Logf("初始分片状态验证完成: 所有分片状态符合预期")
		}

		// 5. 更新分片计划：移除shard-2
		newShardPlan := []string{"shard-1", "shard-3", "shard-4"}
		t.Logf("更新分片计划: %v -> %v", initialShardPlan, newShardPlan)
		setShardPlan(t, setup.FakeEtcd, ctx, newShardPlan)

		// 6. 等待分片状态更新

		// 期望的更新后状态：新计划内分片为非lameDuck，计划外分片为lameDuck
		updatedExpectedStates := map[data.ShardId]ExpectedShardState{
			"shard-1": {LameDuck: false}, // 仍在计划内
			"shard-2": {LameDuck: true},  // 已从计划中移除
			"shard-3": {LameDuck: false}, // 仍在计划内
			"shard-4": {LameDuck: false}, // 新加入计划
		}
		t.Logf("等待分片状态根据新计划更新...")
		{
			waitSucc, elapsedMs := WaitUntil(t, func() (bool, string) {
				results := verifyAllShardState(t, ss, updatedExpectedStates)
				return len(results) == 0, strings.Join(results, ",")
			}, 1000, 100)
			assert.True(t, waitSucc, "应该能在超时前根据新计划更新所有分片状态, 耗时=%dms", elapsedMs)
			t.Logf("分片状态已根据新计划更新, 耗时=%dms", elapsedMs)
		}

		// 7. 特别验证shard-2和shard-4的状态变化
		{
			var shard2State, shard4State *ShardState
			var found2, found4 bool
			safeAccessServiceState(ss, func(ss *ServiceState) {
				shard2State, found2 = ss.AllShards["shard-2"]
				shard4State, found4 = ss.AllShards["shard-4"]
			})

			assert.True(t, found2, "应能找到shard-2")
			assert.True(t, found4, "应能找到shard-4")

			if found2 && found4 {
				assert.True(t, shard2State.LameDuck, "shard-2应变为lameDuck状态")
				assert.False(t, shard4State.LameDuck, "shard-4应变为非lameDuck状态")
				t.Logf("验证成功：shard-2已变为lameDuck状态，shard-4已变为非lameDuck状态")
			}
		}

		// 8. 验证分片状态持久化
		{
			waitSucc, elapsedMs := WaitUntil(t, func() (bool, string) {
				results := verifyAllShardsInStorage(t, setup, updatedExpectedStates)
				if len(results) > 0 {
					return false, strings.Join(results, ",")
				}
				return true, ""
			}, 100, 10)
			assert.True(t, waitSucc, "分片状态持久化验证应该全部通过, 耗时=%dms", elapsedMs)
			t.Logf("分片状态持久化验证完成，耗时=%dms", elapsedMs)
		}

		// 结束前停止 ServiceState
		ss.StopAndWaitForExit(ctx)
		t.Logf("ServiceState已停止，测试完成")
	}

	// 使用 FakeTimeProvider 和模拟的 EtcdProvider/EtcdStore 运行测试
	setup.RunWith(fn)
}

// setShardPlan 设置分片计划
func setShardPlan(t *testing.T, fakeEtcd *etcdprov.FakeEtcdProvider, ctx context.Context, shardNames []string) {
	shardPlanStr := ""
	for i, name := range shardNames {
		if i > 0 {
			shardPlanStr += "\n"
		}
		shardPlanStr += name
	}
	fakeEtcd.Set(ctx, "/smg/config/shard_plan.txt", shardPlanStr)
}
