package core

import (
	"context"
	"log/slog"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/libs/unicorn/unicorn"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
)

func TestSimWorker_basic3(t *testing.T) {
	ctx := context.Background()

	// 配置测试环境
	setup := NewFakeTimeTestSetup(t)
	setup.SetupBasicConfig(ctx, config.WithSolverConfig(func(sc *config.SolverConfig) {
		sc.SoftSolverConfig.SolverEnabled = true
		sc.SoftSolverConfig.RunPerMinute = 200
		sc.AssignSolverConfig.SolverEnabled = true
		sc.AssignSolverConfig.RunPerMinute = 200
		sc.UnassignSolverConfig.SolverEnabled = true
	}))
	slog.InfoContext(ctx, "",
		slog.String("event", "测试环境已配置"))

	fn := func() {
		// Step 1: 创建 shardPlan and set into etcd
		// 添加2个分片 (shard_1/2)
		slog.InfoContext(ctx, "创建 shardPlan",
			slog.String("event", "Step1"))
		firstShardPlan := unicorn.RangeBasedShardIdGenerateString("shard", 32, 2)
		setup.SetShardPlan(ctx, firstShardPlan)

		// Step 2: 创建 ServiceState
		slog.InfoContext(ctx, "创建 ServiceState",
			slog.String("event", "Step2"))
		ss := AssembleSsAll(ctx, "TestAssembleAssignSolver")
		setup.ServiceState = ss
		slog.InfoContext(ctx, ss.Name,
			slog.String("event", "ServiceState已创建"))

		// Step 3: 创建 worker-1 eph
		slog.InfoContext(ctx, "创建 worker-1 eph",
			slog.String("event", "Step3"))
		var workers []*FakeWorker
		for i := range 10 {
			workerName := "worker-" + strconv.Itoa(i+1)
			sessionId := "session-" + strconv.Itoa(i+1)
			workers = append(workers, NewFakeWorker(t, setup, workerName, sessionId, "localhost:808"+strconv.Itoa(i+1)))
		}

		{
			// step 4: wait assign to happen
			slog.InfoContext(ctx, "wait for move to happen",
				slog.String("event", "Step4"))
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
			slog.InfoContext(ctx, "worker-1 shutdown request",
				slog.String("event", "Step5"))
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
		slog.InfoContext(ctx, "end of test",
			slog.String("event", "Step6"))
	}

	// 使用 FakeTimeProvider 和模拟的 EtcdProvider/EtcdStore 运行测试
	setup.RunWith(fn)
}
