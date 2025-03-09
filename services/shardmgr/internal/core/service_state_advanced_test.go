package core

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/etcdprov"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/shadow"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
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
				ss := NewServiceState(ctx)
				success := waitServiceShards(t, ss, 3)
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
				ss := NewServiceState(ctx)
				success := waitServiceShards(t, ss, 3)
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
				ss := NewServiceState(ctx)
				success := waitServiceShards(t, ss, 4) // 应该有4个分片（包括lameDuck的shard-4）
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
				success = waitServiceShards(t, ss, 4) // 仍然有4个分片
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

// 以下是测试辅助函数

// setupBasicConfig 设置基本配置
func setupBasicConfig(t *testing.T, fakeEtcd *etcdprov.FakeEtcdProvider, ctx context.Context) {
	// 创建服务信息
	serviceInfo := smgjson.CreateTestServiceInfo()
	fakeEtcd.Set(ctx, "/smg/config/service_info.json", serviceInfo.ToJson())

	// 创建服务配置
	serviceConfig := smgjson.CreateTestServiceConfig()
	fakeEtcd.Set(ctx, "/smg/config/service_config.json", serviceConfig.ToJson())
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

// createPreExistingShards 创建预先存在的分片状态
func createPreExistingShards(t *testing.T, fakeEtcd *etcdprov.FakeEtcdProvider, ctx context.Context, shardStates map[string]bool) {
	pm := config.NewPathManager()

	for shardName, isLameDuck := range shardStates {
		// 使用正确的类型转换
		shardId := data.ShardId(shardName)

		// 创建一个 ShardStateJson 结构
		shardStateJson := &smgjson.ShardStateJson{
			ShardName: shardId,
		}

		if isLameDuck {
			shardStateJson.LameDuck = 1
		} else {
			shardStateJson.LameDuck = 0
		}

		// 转换为 JSON 并存储
		jsonStr := shardStateJson.ToJson()
		fakeEtcd.Set(ctx, pm.FmtShardStatePath(shardId), jsonStr)
	}
}

// waitServiceShards 等待ServiceState的分片数量达到预期
func waitServiceShards(t *testing.T, ss *ServiceState, expectedCount int) bool {
	t.Helper()

	// 使用waitForServiceShards，这是标准实现
	result, _ := waitForServiceShards(t, ss, expectedCount)

	return result
}

// serviceStateReadEvent 用于安全读取 ServiceState
type serviceStateReadEvent struct {
	callback func(*ServiceState)
}

// GetName 返回事件名称
func (e *serviceStateReadEvent) GetName() string {
	return "ServiceStateRead"
}

// Process 处理事件
func (e *serviceStateReadEvent) Process(ctx context.Context, ss *ServiceState) {
	e.callback(ss)
}

// verifyAllShards 验证所有分片的状态
func verifyAllShards(t *testing.T, ss *ServiceState, expectedStates map[data.ShardId]bool) {
	// 创建通道，用于接收验证结果
	resultChan := make(chan bool)
	errorsChan := make(chan string, 10)

	// 创建事件来安全访问 ServiceState
	ss.EnqueueEvent(&serviceStateReadEvent{
		callback: func(s *ServiceState) {
			// 首先输出当前所有分片状态，方便调试
			t.Logf("当前存在 %d 个分片, 需要验证 %d 个", len(s.AllShards), len(expectedStates))
			for shardId, shard := range s.AllShards {
				t.Logf("    - 分片 %s 状态: lameDuck=%v", shardId, shard.LameDuck)
			}

			// 验证每个期望的分片
			allPassed := true
			for shardId, expectedLameDuck := range expectedStates {
				shard, ok := s.AllShards[shardId]
				if !ok {
					errorMsg := fmt.Sprintf("未找到分片 %s", shardId)
					t.Error(errorMsg)
					errorsChan <- errorMsg
					allPassed = false
					continue // 继续检查其他分片
				}

				if shard.LameDuck != expectedLameDuck {
					errorMsg := fmt.Sprintf("分片 %s 的 lameDuck 状态不符合预期，期望: %v，实际: %v",
						shardId, expectedLameDuck, shard.LameDuck)
					t.Error(errorMsg)
					errorsChan <- errorMsg
					allPassed = false
					continue // 继续检查其他分片
				}

				t.Logf("分片 %s 状态验证通过: lameDuck=%v", shardId, shard.LameDuck)
			}

			// 所有验证都通过
			resultChan <- allPassed
		},
	})

	// 等待验证结果
	success := <-resultChan

	// 读取错误通道中的所有信息（已在回调中输出）
	// 清空通道
	select {
	case <-errorsChan:
		// 继续清空通道
	default:
		// 通道已空
	}

	assert.True(t, success, "分片状态验证应该通过")
}
