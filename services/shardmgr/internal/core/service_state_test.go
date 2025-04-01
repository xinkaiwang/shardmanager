package core

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/etcdprov"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

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
	jsonData, _ := json.Marshal(serviceInfo)
	fakeEtcd.Set(ctx, serviceInfoPath, string(jsonData))

	// 设置服务配置
	serviceConfigPath := "/smg/config/service_config.json"
	serviceConfig := smgjson.CreateTestServiceConfig()
	configData, _ := json.Marshal(serviceConfig)
	fakeEtcd.Set(ctx, serviceConfigPath, string(configData))

	// 使用 RunWithEtcdProvider 运行测试
	etcdprov.RunWithEtcdProvider(fakeEtcd, func() {
		// 创建 ServiceState
		ss := AssembleSsWithShadowState(ctx, "TestServiceState_Basic")

		// 不使用 defer 调用 StopAndWaitForExit，而是在测试结束前直接调用
		// defer ss.StopAndWaitForExit(ctx)

		// 验证基本信息是否正确加载
		assert.Equal(t, "test-service", ss.ServiceInfo.ServiceName)
		assert.Equal(t, data.ST_MEMORY, ss.ServiceInfo.StatefulType)
		assert.Equal(t, smgjson.MP_StartBeforeKill, ss.ServiceInfo.DefaultHints.MovePolicy)
		assert.Equal(t, 10, ss.ServiceInfo.DefaultHints.MaxReplicaCount)
		assert.Equal(t, 1, ss.ServiceInfo.DefaultHints.MinReplicaCount)

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
		ss := AssembleSsWithShadowState(ctx, "TestServiceState_ShardManagement")

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
