package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

func TestAssembleAssignSolver(t *testing.T) {
	ctx := context.Background()

	// 配置测试环境
	setup := NewFakeTimeTestSetup(t)
	setup.SetupBasicConfig(ctx, config.WithSolverConfig(func(sc *config.SolverConfig) {
		sc.SoftSolverConfig.SolverEnabled = false
		sc.AssignSolverConfig.SolverEnabled = false
		sc.UnassignSolverConfig.SolverEnabled = false
	}))
	klogging.Info(ctx).Log("测试环境已配置", "")

	fn := func() {
		// Step 1: 创建 shardPlan and set into etcd
		// 添加2个分片 (shard_1, shard_2)
		firstShardPlan := []string{"shard_1", "shard_2"}
		setup.SetShardPlan(ctx, firstShardPlan)

		// Step 2: 创建 ServiceState
		ss := AssembleSsAll(ctx, "TestAssembleAssignSolver")
		setup.ServiceState = ss
		klogging.Info(ctx).Log("ServiceState已创建", ss.Name)

		{
			// 等待快照更新
			waitSucc, elapsedMs := setup.WaitUntilSs(t, func(ss *ServiceState) (bool, string) {
				if ss.SnapshotFuture == nil {
					return false, "快照不存在"
				}
				if !ss.SnapshotFuture.GetCost().IsEqualTo(costfunc.NewCost(2, 0.0)) {
					return false, "快照不正确"
				}
				return true, "" // 快照存在
			}, 1000, 10)
			assert.True(t, waitSucc, "应该能在超时前创建快照, 耗时=%dms", elapsedMs)
		}
		// Step 3: 创建 worker-1 eph
		workerFullId, _ := setup.CreateAndSetWorkerEph(t, "worker-1", "session-1", "localhost:8080")

		{
			// 等待worker state创建
			waitSucc, elapsedMs := setup.WaitUntilWorkerStateEnum(t, workerFullId, data.WS_Online_healthy, 1000, 10)
			assert.Equal(t, true, waitSucc, "worker state 已创建 elapsedMs=%d", elapsedMs)
		}

		// assert.Equal(t, true, false, "") // 强制查看测试输出
	}

	// 使用 FakeTimeProvider 和模拟的 EtcdProvider/EtcdStore 运行测试
	setup.RunWith(fn)
}
