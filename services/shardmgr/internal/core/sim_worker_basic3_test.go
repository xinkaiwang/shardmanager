package core

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/libs/unicorn/unicorn"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
)

func TestSimWorker_basic3(t *testing.T) {
	ctx := context.Background()
	klogging.SetDefaultLogger(klogging.NewLogrusLogger(ctx).SetConfig(ctx, "debug", "simple"))

	// 配置测试环境
	setup := NewFakeTimeTestSetup(t)
	setup.SetupBasicConfig(ctx, config.WithSolverConfig(func(sc *config.SolverConfig) {
		sc.SoftSolverConfig.SolverEnabled = true
		sc.SoftSolverConfig.RunPerMinute = 200
		sc.AssignSolverConfig.SolverEnabled = true
		sc.AssignSolverConfig.RunPerMinute = 200
		sc.UnassignSolverConfig.SolverEnabled = true
	}))
	klogging.Info(ctx).Log("测试环境已配置", "")

	fn := func() {
		// Step 1: 创建 shardPlan and set into etcd
		// 添加2个分片 (shard_1/2)
		klogging.Info(ctx).Log("Step1", "创建 shardPlan")
		firstShardPlan := unicorn.RangeBasedShardIdGenerateString("shard", 32, 2)
		setup.SetShardPlan(ctx, firstShardPlan)

		// Step 2: 创建 ServiceState
		klogging.Info(ctx).Log("Step2", "创建 ServiceState")
		ss := AssembleSsAll(ctx, "TestAssembleAssignSolver")
		setup.ServiceState = ss
		klogging.Info(ctx).Log("ServiceState已创建", ss.Name)

		// Step 3: 创建 worker-1 eph
		klogging.Info(ctx).Log("Step3", "创建 worker-1 eph")
		var workers []*FakeWorker
		for i := range 10 {
			workerName := "worker-" + strconv.Itoa(i+1)
			sessionId := "session-" + strconv.Itoa(i+1)
			workers = append(workers, NewFakeWorker(t, setup, workerName, sessionId, "localhost:808"+strconv.Itoa(i+1)))
		}

		{
			// step 4: wait assign to happen
			klogging.Info(ctx).Log("Step4", "wait for move to happen") // smg will assign shard_1 to worker-1, and worker-1 will become accept it.
			waitSucc, elapsedMs := setup.WaitUntilSs(t, func(ss *ServiceState) (bool, string) {
				cost := ss.SnapshotCurrent.GetCost(ctx)
				if cost.HardScore != 0 {
					return false, "hard score =" + strconv.Itoa(int(cost.HardScore))
				}
				return true, "hard score 为0"
			}, 30*1000, 1000)
			assert.Equal(t, true, waitSucc, "应该能在超时前完成 assign, 耗时=%dms", elapsedMs)
		}
		setup.FakeTime.VirtualTimeForward(ctx, 30*1000)
		{
			// step 5: worker-1 shutdown request
			klogging.Info(ctx).Log("Step5", "worker-1 shutdown request")
			workers[0].RequestShutdown()
			waitSucc, elapsedMs := WaitUntil(t, func() (bool, string) {
				if workers[0].stopped {
					return true, "worker-1 已经停止"
				}
				return false, "worker-1 还没有停止"
			}, 60*1000, 1000)
			assert.Equal(t, true, waitSucc, "应该能在超时前完成 worker-1 shutdown request, 耗时=%dms", elapsedMs)
		}
		setup.FakeTime.VirtualTimeForward(ctx, 61*1000)
		klogging.Info(ctx).Log("Step6", "end of test")
	}

	// 使用 FakeTimeProvider 和模拟的 EtcdProvider/EtcdStore 运行测试
	setup.RunWith(fn)
}
