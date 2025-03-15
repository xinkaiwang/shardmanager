package core

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/etcdprov"
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
			assert.True(t, allPassed, "分片状态验证应该全部通过")
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
		results := verifyAllShardsInStorage(t, setup, expectedShardStates)
		assert.Equal(t, 0, len(results), "分片状态持久化验证应该全部通过, result=%v", results)

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
	// TODO
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
	// TODO
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
