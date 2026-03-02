package core

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/libs/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/libs/unicorn/unicornjson"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
)

func TestAssembleShardPlan(t *testing.T) {
	ctx := context.Background()

	// 配置测试环境
	setup := NewFakeTimeTestSetup(t)
	setup.SetupBasicConfig(ctx, config.WithSolverConfig(func(sc *config.SolverConfig) {
		sc.SoftSolverConfig.SolverEnabled = true
		sc.AssignSolverConfig.SolverEnabled = true
		sc.UnassignSolverConfig.SolverEnabled = true
	}))
	slog.InfoContext(ctx, "",
		slog.String("event", "测试环境已配置"))

	fn := func() {
		// Step 1: 创建 ServiceState
		slog.InfoContext(ctx, "创建 ServiceState",
			slog.String("event", "Step1"))
		ss := AssembleSsAll(ctx, "TestAssembleAssignSolver")
		setup.ServiceState = ss
		slog.InfoContext(ctx, ss.Name,
			slog.String("event", "ServiceState已创建"))

		// Step 2: 创建 worker-1 eph
		slog.InfoContext(ctx, "创建 worker-1 eph",
			slog.String("event", "Step2"))
		workerFullId, _ := setup.CreateAndSetWorkerEph(t, "worker-1", "session-1", "localhost:8080")

		slog.InfoContext(ctx, "创建 shardPlan",
			slog.String("event", "Step3"))
		setup.SetShardPlan(ctx, []string{"shard_1"})

		{
			var pilotAssign *cougarjson.PilotAssignmentJson
			waitSucc, elapsedMs := setup.WaitUntilPilotNode(t, workerFullId, func(pnj *cougarjson.PilotNodeJson) (bool, string) {
				if pnj == nil {
					return false, "没有 pilot 节点"
				}
				if len(pnj.Assignments) == 1 {
					pilotAssign = pnj.Assignments[0]
					return true, pnj.ToJson()
				}
				return false, pnj.ToJson()
			}, 30*1000, 1000)
			assert.Equal(t, true, waitSucc, "应该能在超时前 pilotNode update, 耗时=%dms", elapsedMs)
			// step4: simulate eph node update
			slog.InfoContext(ctx, "simulate eph node update",
				slog.String("event", "Step4"))
			setup.UpdateEphNode(workerFullId, func(wej *cougarjson.WorkerEphJson) *cougarjson.WorkerEphJson {
				assign := cougarjson.NewAssignmentJson(pilotAssign.ShardId, pilotAssign.ReplicaIdx, pilotAssign.AssignmentId, cougarjson.CAS_Ready)
				wej.Assignments = append(wej.Assignments, assign)
				wej.LastUpdateAtMs = kcommon.GetWallTimeMs()
				wej.LastUpdateReason = "simulate eph node update"
				return wej
			})
		}

		{
			waitSucc, elapsedMs := setup.WaitUntilRoutingState(t, workerFullId, func(rj *unicornjson.WorkerEntryJson) (bool, string) {
				if rj == nil {
					return false, "没有 routing 节点"
				}
				if len(rj.Assignments) == 1 {
					return true, rj.ToJson()
				}
				return false, rj.ToJson()
			}, 1*1000, 100)
			assert.Equal(t, true, waitSucc, "应该能在超时前 show up in routing, 耗时=%dms", elapsedMs)
		}

		// Step 5: Remove shardPlan
		slog.InfoContext(ctx, "Remove shardPlan",
			slog.String("event", "Step5"))
		setup.SetShardPlan(ctx, []string{})
		setup.FakeTime.VirtualTimeForward(ctx, 60*1000)
		{
			waitSucc, elapsedMs := setup.WaitUntilPilotNode(t, workerFullId, func(pnj *cougarjson.PilotNodeJson) (bool, string) {
				if pnj == nil {
					return false, "没有 pilot 节点"
				}
				if len(pnj.Assignments) == 0 {
					return true, pnj.ToJson()
				}
				return false, pnj.ToJson()
			}, 30*1000, 1000)
			assert.Equal(t, true, waitSucc, "应该能在超时前 pilotNode update, 耗时=%dms", elapsedMs)
			// step6: simulate eph node update
			slog.InfoContext(ctx, "simulate eph node update",
				slog.String("event", "Step6"))
			setup.UpdateEphNode(workerFullId, func(wej *cougarjson.WorkerEphJson) *cougarjson.WorkerEphJson {
				wej.Assignments = nil
				wej.LastUpdateAtMs = kcommon.GetWallTimeMs()
				wej.LastUpdateReason = "simulate eph node update"
				return wej
			})
		}

		// Step 7:
		{
			slog.InfoContext(ctx, "wait until no more moves",
				slog.String("event", "Step7"))
			waitSucc, elapsedMs := setup.WaitUntilSs(t, func(ss *ServiceState) (bool, string) {
				if ss == nil {
					return false, "没有 ServiceState"
				}
				if len(ss.AllMoves) == 0 {
					return true, "没有 Moves"
				}
				return false, "有 Moves"
			}, 30*1000, 1000)
			assert.Equal(t, true, waitSucc, "应该能在超时前 show up in ServiceState, 耗时=%dms", elapsedMs)
		}

		// step 8: wait for 60s
		slog.InfoContext(ctx, "wait for 60s",
			slog.String("event", "Step8"))
		setup.FakeTime.VirtualTimeForward(ctx, 60*1000)
	}

	// 使用 FakeTimeProvider 和模拟的 EtcdProvider/EtcdStore 运行测试
	setup.RunWith(fn)
}
