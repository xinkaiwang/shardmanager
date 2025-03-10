package core

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/etcdprov"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/shadow"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

// ServiceStateTestSetup 包含服务状态测试所需的基本设置
type ServiceStateTestSetup struct {
	Context       context.Context
	FakeEtcd      *etcdprov.FakeEtcdProvider
	FakeStore     *shadow.FakeEtcdStore
	ServiceState  *ServiceState
	InitialShards map[data.ShardId]bool // 初始分片及其 LameDuck 状态 (true = lameDuck)
	TestLock      sync.RWMutex          // 用于测试助手的锁
}

// CreateTestSetup 创建基本的测试环境
func CreateTestSetup(t *testing.T) *ServiceStateTestSetup {
	resetGlobalState(t)

	fakeEtcd := etcdprov.NewFakeEtcdProvider()
	fakeStore := shadow.NewFakeEtcdStore()

	ctx := context.Background()

	return &ServiceStateTestSetup{
		Context:       ctx,
		FakeEtcd:      fakeEtcd,
		FakeStore:     fakeStore,
		InitialShards: make(map[data.ShardId]bool),
	}
}

// SetupBasicConfig 设置基本配置
func (s *ServiceStateTestSetup) SetupBasicConfig(t *testing.T) {
	// 准备服务信息
	serviceInfo := smgjson.CreateTestServiceInfo()
	s.FakeEtcd.Set(s.Context, "/smg/config/service_info.json", serviceInfo.ToJson())

	// 准备服务配置
	serviceConfig := smgjson.CreateTestServiceConfig()
	s.FakeEtcd.Set(s.Context, "/smg/config/service_config.json", serviceConfig.ToJson())
}

// SetShardPlan 设置分片计划
func (s *ServiceStateTestSetup) SetShardPlan(t *testing.T, shardNames []string) {
	shardPlanStr := ""
	for i, name := range shardNames {
		if i > 0 {
			shardPlanStr += "\n"
		}
		shardPlanStr += name
	}
	s.FakeEtcd.Set(s.Context, "/smg/config/shard_plan.txt", shardPlanStr)
}

// CreatePreExistingShards 创建预先存在的分片状态
func (s *ServiceStateTestSetup) CreatePreExistingShards(t *testing.T, shardStates map[string]bool) {
	pm := config.NewPathManager()

	for shardName, isLameDuck := range shardStates {
		// 使用正确的类型转换
		shardId := data.ShardId(shardName)

		// 创建一个ShardStateJson - 注意：ShardName应该是 data.ShardId 类型
		shardStateJson := &smgjson.ShardStateJson{
			ShardName: shardId, // 正确使用 data.ShardId 类型
		}

		// 设置 LameDuck 状态
		if isLameDuck {
			shardStateJson.LameDuck = 1
		} else {
			shardStateJson.LameDuck = 0
		}

		// 转换为JSON并存储
		jsonStr := shardStateJson.ToJson()
		s.FakeEtcd.Set(s.Context, pm.FmtShardStatePath(shardId), jsonStr)

		// 记录到InitialShards
		s.InitialShards[shardId] = isLameDuck
	}
}

// CreateServiceState 创建 ServiceState 并等待分片加载完成
func (s *ServiceStateTestSetup) CreateServiceState(t *testing.T, expectedShardCount int) {
	s.ServiceState = NewServiceState(s.Context)
	success, waitDuration := waitForServiceShards(t, s.ServiceState, expectedShardCount)
	assert.True(t, success, "应该能在超时前加载分片状态")
	t.Logf("加载分片状态等待时间: %v", waitDuration)
}

// VerifyShardState 验证分片状态
func (s *ServiceStateTestSetup) VerifyShardState(t *testing.T, shardName string, expectedLameDuck bool) {
	shardId := data.ShardId(shardName)

	// 使用安全方式获取分片状态
	var shard *ShardState
	var ok bool

	withServiceStateSync(s.ServiceState, func(ss *ServiceState) {
		shard, ok = ss.AllShards[shardId]
	})

	assert.True(t, ok, "应该能找到分片 %s", shardName)
	if ok {
		assert.Equal(t, expectedLameDuck, shard.LameDuck, "分片 %s 的 lameDuck 状态不符合预期", shardName)
	}
}

// UpdateShardPlan 更新分片计划并等待更新完成
func (s *ServiceStateTestSetup) UpdateShardPlan(t *testing.T, shardNames []string, expectedShardCount int) {
	s.SetShardPlan(t, shardNames)
	success, waitDuration := waitForServiceShards(t, s.ServiceState, expectedShardCount)
	assert.True(t, success, "应该能在超时前更新分片状态")
	t.Logf("更新分片等待时间: %v", waitDuration)
}

// 安全地访问 ServiceState 内部状态（同步方式）
func withServiceStateSync(ss *ServiceState, fn func(*ServiceState)) {
	// 创建同步通道
	completed := make(chan struct{})

	// 创建一个事件并加入队列
	ss.EnqueueEvent(&serviceStateAccessEvent{
		callback: fn,
		done:     completed,
	})

	// 等待事件处理完成
	<-completed
}

// serviceStateAccessEvent 是一个用于访问 ServiceState 的事件
type serviceStateAccessEvent struct {
	callback func(*ServiceState)
	done     chan struct{}
}

// GetName 返回事件名称
func (e *serviceStateAccessEvent) GetName() string {
	return "ServiceStateAccess"
}

// Process 实现 IEvent 接口
func (e *serviceStateAccessEvent) Process(ctx context.Context, ss *ServiceState) {
	e.callback(ss)
	close(e.done)
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
// 返回是否成功和等待时间
func waitForServiceShards(t *testing.T, ss *ServiceState, expectedCount int) (bool, time.Duration) {
	startTime := time.Now()
	result := WaitUntil(t, func() (bool, string) {
		if len(ss.AllShards) >= expectedCount {
			t.Logf("ServiceState 中的分片数量已达到预期：%d", len(ss.AllShards))
			return true, ""
		}
		var shardIds []string
		for id := range ss.AllShards {
			shardIds = append(shardIds, string(id))
		}
		return false, fmt.Sprintf("当前分片数量: %d, 分片列表: %v", len(ss.AllShards), shardIds)
	}, 2000, 20)

	waitDuration := time.Since(startTime)
	t.Logf("waitForServiceShards 总等待时间: %v，结果: %v", waitDuration, result)
	return result, waitDuration
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
	close(errorsChan)

	// 返回验证结果
	assert.True(t, success, "分片状态验证应该全部通过")
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
