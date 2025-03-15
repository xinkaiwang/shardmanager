package core

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/etcdprov"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

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
		ss := NewServiceState(ctx, "TestShardBasic_ConsistencyCheck")
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
			waitSucc, elapsedMs := WaitUntilShardCount(t, ss, 3, 1000, 100)
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
		ss := NewServiceState(ctx, "TestShardBasic_ConflictResolution")
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
			waitSucc, elapsedMs := WaitUntilShardCount(t, ss, 3, 1000, 100)
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
		ss := NewServiceState(ctx, "TestShardBasic_DynamicPlanUpdate")
		setup.ServiceState = ss
		t.Logf("ServiceState已创建: %s", ss.Name)

		// 1. 设置初始分片计划，包含3个分片
		initialShardPlan := []string{"shard-1", "shard-2", "shard-3"}
		setShardPlan(t, setup.FakeEtcd, ctx, initialShardPlan)
		t.Logf("初始分片计划已设置：%v", initialShardPlan)

		// 2. 等待ServiceState加载分片状态
		t.Logf("等待ServiceState加载初始分片状态...")
		{
			waitSucc, elapsedMs := WaitUntilShardCount(t, ss, 3, 1000, 100)
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
			waitSucc, elapsedMs := WaitUntilShardCount(t, ss, 4, 1000, 100)
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
