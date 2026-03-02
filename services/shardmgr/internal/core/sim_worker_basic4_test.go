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

func TestSimWorker_basic4(t *testing.T) {
	ctx := context.Background()

	// 配置测试环境
	setup := NewFakeTimeTestSetup(t)
	setup.SetupBasicConfig(ctx, config.WithSolverConfig(func(sc *config.SolverConfig) {
		sc.SoftSolverConfig.SolverEnabled = true
		sc.SoftSolverConfig.RunPerMinute = 200
		sc.AssignSolverConfig.SolverEnabled = true
		sc.AssignSolverConfig.RunPerMinute = 200
		sc.UnassignSolverConfig.SolverEnabled = true
	}), config.WithSystemLimitFn(func(sl *config.SystemLimitConfig) {
		sl.MaxHatCountLimit = 2
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
			workerName := "worker-" + strconv.Itoa(i)
			sessionId := "session-" + strconv.Itoa(i)
			workers = append(workers, NewFakeWorker(t, setup, workerName, sessionId, "localhost:808"+strconv.Itoa(i)))
		}

		{
			// step 4: wait assign to happen
			slog.InfoContext(ctx, "wait for move to happen",
				slog.String("event", "Step4"))
			waitSucc, elapsedMs := setup.WaitUntilSs(t, func(ss *ServiceState) (bool, string) {
				if ss.SnapshotCurrent.GetCost(ctx).HardScore != 0 {
					return false, "hard score 不为0"
				}
				return true, "hard score 为0"
			}, 30*1000, 1000)
			assert.Equal(t, true, waitSucc, "应该能在超时前完成 assign, 耗时=%dms", elapsedMs)
		}
		setup.FakeTime.VirtualTimeForward(ctx, 30*1000)
		{
			// step 5: worker-1 shutdown request
			slog.InfoContext(ctx, "worker-1/2/3/4 shutdown request",
				slog.String("event", "Step5"))
			workers[0].RequestShutdown()
			workers[1].RequestShutdown()
			workers[2].RequestShutdown()
			workers[3].RequestShutdown()
			{
				waitSucc, elapsedMs := WaitUntil(t, func() (bool, string) {
					stoppedCount := CountFakeWorkers(workers, func(w *FakeWorker) bool {
						return w.stopped
					})
					if stoppedCount > 0 {
						return true, "2 out of 4 workers 已经停止:" + strconv.Itoa(stoppedCount)
					}
					return false, "2 worker 还没有停止:" + strconv.Itoa(stoppedCount)
				}, 360*1000, 1000)
				assert.Equal(t, true, waitSucc, "应该能在超时前完成 worker-1 shutdown request, 耗时=%dms", elapsedMs)
			}
			{
				waitSucc, elapsedMs := WaitUntil(t, func() (bool, string) {
					stoppedCount := CountFakeWorkers(workers, func(w *FakeWorker) bool {
						return w.stopped
					})
					if stoppedCount >= 4 {
						return true, "4 out of 4 workers 已经停止"
					}
					return false, "stop_count=" + strconv.Itoa(stoppedCount)
				}, 120*1000, 1000)
				assert.Equal(t, true, waitSucc, "应该能在超时前完成 worker-2 shutdown request, 耗时=%dms", elapsedMs)
			}
		}
		setup.FakeTime.VirtualTimeForward(ctx, 60*1000)
		slog.InfoContext(ctx, "end of test",
			slog.String("event", "Step6"))
	}

	// 使用 FakeTimeProvider 和模拟的 EtcdProvider/EtcdStore 运行测试
	setup.RunWith(fn)
}
