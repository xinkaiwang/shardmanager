package core

import (
	"context"

	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/etcdprov"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/shadow"
)

// TestServiceState_StateConflictResolution 测试当分片状态与分片计划冲突时的解决方案
func TestServiceState_StateConflictResolution(t *testing.T) {
	// 重置全局状态，确保测试环境干净
	resetGlobalState(t)

	// 创建测试环境
	fakeEtcd := etcdprov.NewFakeEtcdProvider()
	fakeStore := shadow.NewFakeEtcdStore()

	etcdprov.RunWithEtcdProvider(fakeEtcd, func() {
		shadow.RunWithEtcdStore(fakeStore, func() {
			ctx := context.Background()

			// 设置基本配置
			setupBasicConfig(t, fakeEtcd, ctx)

			// 测试一：一致性验证 - 初始分片状态与分片计划一致
			t.Run("ConsistencyCheck", func(t *testing.T) {
				// 设置初始分片计划：3个分片
				shardPlan := []string{"shard-1", "shard-2", "shard-3"}
				setShardPlan(t, fakeEtcd, ctx, shardPlan)

				// 创建与分片计划一致的分片状态
				shardStates := map[string]bool{
					"shard-1": false, // 正常状态
					"shard-2": false,
					"shard-3": false,
				}
				createPreExistingShards(t, fakeEtcd, ctx, shardStates)

				// 创建服务状态
				ss := NewServiceState(ctx, "ConsistencyCheck")
				success, _ := waitForServiceShards(t, ss, 3)
				assert.True(t, success, "应该能在超时前加载预先存在的分片状态")

				// 验证所有分片状态都是正确的
				verifyAllShards(t, ss, map[data.ShardId]bool{
					"shard-1": false,
					"shard-2": false,
					"shard-3": false,
				})

				// 结束前停止 ServiceState
				ss.StopAndWaitForExit(ctx)
			})

			// 测试二：冲突解决 - 某些分片在 etcd 中标记为 lameDuck，但在分片计划中存在
			t.Run("ConflictResolution", func(t *testing.T) {
				// 设置初始分片计划：3个分片
				shardPlan := []string{"shard-1", "shard-2", "shard-3"}
				setShardPlan(t, fakeEtcd, ctx, shardPlan)

				// 创建与分片计划有冲突的分片状态
				shardStates := map[string]bool{
					"shard-1": false, // 正常状态
					"shard-2": true,  // lameDuck，但在分片计划中
					"shard-3": false, // 正常状态
				}
				createPreExistingShards(t, fakeEtcd, ctx, shardStates)

				// 创建服务状态
				ss := NewServiceState(ctx, "ConflictResolution")
				success, _ := waitForServiceShards(t, ss, 3)
				assert.True(t, success, "应该能在超时前加载预先存在的分片状态")

				// 验证所有分片状态都被正确调整
				verifyAllShards(t, ss, map[data.ShardId]bool{
					"shard-1": false,
					"shard-2": false, // 虽然在etcd中是lameDuck，但应被调整为非lameDuck
					"shard-3": false,
				})

				// 结束前停止 ServiceState
				ss.StopAndWaitForExit(ctx)
			})

			// 测试三：动态计划更新 - 分片计划变化后的状态更新
			t.Run("DynamicPlanUpdate", func(t *testing.T) {
				// 设置初始分片计划：3个分片
				shardPlan := []string{"shard-1", "shard-2", "shard-3"}
				setShardPlan(t, fakeEtcd, ctx, shardPlan)

				// 创建初始分片状态
				shardStates := map[string]bool{
					"shard-1": false,
					"shard-2": false,
					"shard-3": false,
					"shard-4": true, // 不在初始计划中，是 lameDuck
				}
				createPreExistingShards(t, fakeEtcd, ctx, shardStates)

				// 创建服务状态
				ss := NewServiceState(ctx, "DynamicPlanUpdate")
				success, _ := waitForServiceShards(t, ss, 4) // 应该有4个分片（包括lameDuck的shard-4）
				assert.True(t, success, "应该能在超时前加载预先存在的分片状态")

				// 验证初始状态
				verifyAllShards(t, ss, map[data.ShardId]bool{
					"shard-1": false,
					"shard-2": false,
					"shard-3": false,
					"shard-4": true, // 不在计划中，应保持lameDuck
				})

				// 更新分片计划：移除 shard-2，添加 shard-4
				updatedShardPlan := []string{"shard-1", "shard-3", "shard-4"}
				setShardPlan(t, fakeEtcd, ctx, updatedShardPlan)

				// 等待分片状态更新
				success, _ = waitForServiceShards(t, ss, 4) // 仍然有4个分片
				assert.True(t, success, "应该能在超时前更新分片状态")

				// 验证分片状态是否正确更新
				verifyAllShards(t, ss, map[data.ShardId]bool{
					"shard-1": false,
					"shard-2": true, // 被移出计划，应变为lameDuck
					"shard-3": false,
					"shard-4": false, // 加入计划，应变为非lameDuck
				})

				// 结束前停止 ServiceState
				ss.StopAndWaitForExit(ctx)
			})
		})
	})
}
