package core

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
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
	FakeTime      *kcommon.FakeTimeProvider
	ServiceState  *ServiceState
	InitialShards map[data.ShardId]bool // 初始分片及其 LameDuck 状态 (true = lameDuck)
	TestLock      sync.RWMutex          // 用于测试助手的锁
}

// CreateTestSetup 创建基本的测试环境
func CreateTestSetup(t *testing.T) *ServiceStateTestSetup {
	resetGlobalState(t)

	fakeEtcd := etcdprov.NewFakeEtcdProvider()
	fakeStore := shadow.NewFakeEtcdStore()
	// 创建并配置 FakeTimeProvider
	fakeTime := kcommon.NewFakeTimeProvider(1234500000000)

	ctx := context.Background()

	setup := &ServiceStateTestSetup{
		Context:       ctx,
		FakeEtcd:      fakeEtcd,
		FakeStore:     fakeStore,
		FakeTime:      fakeTime,
		InitialShards: make(map[data.ShardId]bool),
	}

	return setup
}

func (setup *ServiceStateTestSetup) RunWith(fn func()) {
	etcdprov.RunWithEtcdProvider(setup.FakeEtcd, func() {
		shadow.RunWithEtcdStore(setup.FakeStore, func() {
			kcommon.RunWithTimeProvider(setup.FakeTime, func() {
				fn()
			})
		})
	})
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
	s.ServiceState = NewServiceState(s.Context, "ServiceStateTest")
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

	safeAccessServiceState(s.ServiceState, func(ss *ServiceState) {
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

// waitForServiceShards 等待 ServiceState 中的分片数量达到预期
// 返回是否成功和等待时间ms
func waitForServiceShards(t *testing.T, ss *ServiceState, expectedCount int) (bool, int64) {
	startTime := kcommon.GetMonoTimeMs()

	// 添加一个同步通道，用于在 runloop 中安全地获取分片信息
	type shardInfo struct {
		count int
		ids   []string
	}

	result := WaitUntil(t, func() (bool, string) {
		// 使用 safeAccessServiceState 在 runloop 中安全地访问 ServiceState
		info := make(chan shardInfo, 1)
		safeAccessServiceState(ss, func(ss *ServiceState) {
			var shardIds []string
			for id := range ss.AllShards {
				shardIds = append(shardIds, string(id))
			}
			info <- shardInfo{
				count: len(ss.AllShards),
				ids:   shardIds,
			}
		})

		// 从通道中获取分片信息
		shardData := <-info

		if shardData.count >= expectedCount {
			t.Logf("ServiceState 中的分片数量已达到预期：%d", shardData.count)
			return true, ""
		}

		return false, fmt.Sprintf("当前分片数量: %d, 分片列表: %v", shardData.count, shardData.ids)
	}, 2000, 20)

	waitDuration := kcommon.GetMonoTimeMs() - startTime
	t.Logf("waitForServiceShards 总等待时间: %v，结果: %v", waitDuration, result)
	return result, waitDuration
}

func waitForWorkerCounts(t *testing.T, ss *ServiceState, expectedCount int) (bool, int64) { // 返回是否成功和等待时间ms
	startTime := kcommon.GetMonoTimeMs()
	result := WaitUntil(t, func() (bool, string) {
		if len(ss.AllWorkers) >= expectedCount {
			t.Logf("ServiceState 中的 Worker 数量已达到预期：%d", len(ss.AllWorkers))
			return true, ""
		}
		var workerIds []string
		for id := range ss.AllWorkers {
			workerIds = append(workerIds, id.String())
		}
		return false, fmt.Sprintf("当前 Worker 数量: %d, Worker 列表: %v", len(ss.AllWorkers), workerIds)
	}, 2000, 20)

	waitDuration := kcommon.GetMonoTimeMs() - startTime
	t.Logf("waitForWorkerCounts 总等待时间: %v，结果: %v", waitDuration, result)
	return result, waitDuration
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
	ss.PostEvent(&serviceStateReadEvent{
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

// waitForWorkerStateCreation 等待指定的worker state被创建
// 使用WaitUntil函数实现
func waitForWorkerStateCreation(t *testing.T, ss *ServiceState, workerId string) (bool, int64) {
	t.Helper()

	startMs := kcommon.GetMonoTimeMs()
	fn := func() (bool, string) {
		// 使用 safeAccessServiceState 安全访问 ServiceState
		var workerIds []string
		exists := false

		safeAccessServiceState(ss, func(ss *ServiceState) {
			// 记录所有 worker IDs 用于调试
			workerIds = make([]string, 0, len(ss.AllWorkers))
			for id := range ss.AllWorkers {
				workerIds = append(workerIds, id.String())

				// 检查是否有以指定 workerId 开头的 workerFullId
				if strings.HasPrefix(id.String(), workerId+":") {
					exists = true
				}
			}
		})

		// 生成调试信息字符串
		debugInfo := fmt.Sprintf("查找workerId=%s, workers=%v, exists=%v",
			workerId, workerIds, exists)

		t.Logf("检查worker state: %s", debugInfo)

		return exists, debugInfo
	}
	// 使用已有的WaitUntil函数实现等待
	succ := WaitUntil(t, fn, 1000, 20)
	return succ, kcommon.GetMonoTimeMs() - startMs
}

// waitForWorkerStatePersistence 等待特定路径的worker状态被持久化到存储中
func waitForWorkerStatePersistence(t *testing.T, store *shadow.FakeEtcdStore, path string) (bool, int64) {
	t.Helper()

	startMs := kcommon.GetMonoTimeMs()
	fn := func() (bool, string) {
		// 获取存储数据
		storeData := store.GetData()

		// 检查目标路径是否存在
		value, exists := storeData[path]

		// 生成调试信息
		debugInfo := fmt.Sprintf("路径=%s, 存在=%v, 数据长度=%d, 总存储项数=%d",
			path, exists, len(value), len(storeData))

		// 记录当前尝试的状态
		t.Logf("检查worker状态持久化: %s", debugInfo)

		return exists, debugInfo
	}

	// 使用已有的WaitUntil函数实现等待，最多等待1秒，每50ms检查一次
	succ := WaitUntil(t, fn, 1000, 50)
	return succ, kcommon.GetMonoTimeMs() - startMs
}
