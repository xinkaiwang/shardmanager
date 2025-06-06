package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
)

func TestSimWorker_basic(t *testing.T) {
	ctx := context.Background()
	klogging.SetDefaultLogger(klogging.NewLogrusLogger(ctx).SetConfig(ctx, "debug", "simple"))

	// 配置测试环境
	setup := NewFakeTimeTestSetup(t)
	setup.SetupBasicConfig(ctx, config.WithSolverConfig(func(sc *config.SolverConfig) {
		sc.SoftSolverConfig.SolverEnabled = true
		sc.AssignSolverConfig.SolverEnabled = true
		sc.UnassignSolverConfig.SolverEnabled = true
	}))
	klogging.Info(ctx).Log("测试环境已配置", "")

	fn := func() {
		// Step 1: 创建 shardPlan and set into etcd
		// 添加2个分片 (shard_1/2)
		klogging.Info(ctx).Log("Step1", "创建 shardPlan")
		firstShardPlan := []string{"shard_1"}
		setup.SetShardPlan(ctx, firstShardPlan)

		// Step 2: 创建 ServiceState
		klogging.Info(ctx).Log("Step2", "创建 ServiceState")
		ss := AssembleSsAll(ctx, "TestAssembleAssignSolver")
		setup.ServiceState = ss
		klogging.Info(ctx).Log("ServiceState已创建", ss.Name)

		// Step 3: 创建 worker-1 eph
		klogging.Info(ctx).Log("Step3", "创建 worker-1 eph")
		// worker1 := setup.CreateAndSetWorkerEph(t, "worker-1", "session-1", "localhost:8081")
		NewFakeWorker(t, setup, "worker-1", "session-1", "localhost:8081")

		{
			// step 4: wait assign to happen
			klogging.Info(ctx).Log("Step4", "wait for move to happen") // smg will assign shard_1 to worker-1, and worker-1 will become accept it.
			waitSucc, elapsedMs := setup.WaitUntilSs(t, func(ss *ServiceState) (bool, string) {
				if ss.SnapshotCurrent.GetCost(ctx).HardScore != 0 {
					return false, "hard score 不为0"
				}
				return true, "hard score 为0"
			}, 30*1000, 1000)
			assert.Equal(t, true, waitSucc, "应该能在超时前完成 assign, 耗时=%dms", elapsedMs)
		}
		setup.FakeTime.VirtualTimeForward(ctx, 60*1000)
	}

	// 使用 FakeTimeProvider 和模拟的 EtcdProvider/EtcdStore 运行测试
	setup.RunWith(fn)
}
