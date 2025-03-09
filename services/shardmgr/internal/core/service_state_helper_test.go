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
	success, waitDuration := waitForServiceShardsHelper(t, s.ServiceState, expectedShardCount)
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
	success, waitDuration := waitForServiceShardsHelper(t, s.ServiceState, expectedShardCount)
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

// waitForServiceShardsHelper 等待 ServiceState 中的分片数达到预期数量
func waitForServiceShardsHelper(t *testing.T, ss *ServiceState, expectedCount int) (bool, time.Duration) {
	startTime := time.Now()

	result := WaitUntil(t, func() (bool, string) {
		// 使用安全方式访问 ServiceState
		var shardCount int
		var shardList []string

		withServiceStateSync(ss, func(s *ServiceState) {
			shardCount = len(s.AllShards)
			for shardId := range s.AllShards {
				shardList = append(shardList, string(shardId))
			}
		})

		return shardCount >= expectedCount,
			fmt.Sprintf("当前分片数量: %d, 分片列表: %v", shardCount, shardList)
	}, 2000, 20)

	waitDuration := time.Since(startTime)
	t.Logf("waitForServiceShards 总等待时间: %v，结果: %v", waitDuration, result)
	return result, waitDuration
}
