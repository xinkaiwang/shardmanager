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

func TestAssembleWorkerLifeCycle4(t *testing.T) {
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

		// Step 5: delete worker eph 节点
		slog.InfoContext(ctx, "delete worker eph 节点",
			slog.String("event", "Step5"))
		setup.UpdateEphNode(workerFullId, func(wej *cougarjson.WorkerEphJson) *cougarjson.WorkerEphJson {
			return nil
		})

		slog.InfoContext(ctx, "创建 worker-2 eph",
			slog.String("event", "Step6"))
		workerFullId2, _ := setup.CreateAndSetWorkerEph(t, "worker-2", "session-2", "localhost:8080")

		{
			var pilotAssign2 *cougarjson.PilotAssignmentJson
			waitSucc, elapsedMs := setup.WaitUntilPilotNode(t, workerFullId2, func(pnj *cougarjson.PilotNodeJson) (bool, string) {
				if pnj == nil {
					return false, "没有 pilot 节点"
				}
				if len(pnj.Assignments) == 1 {
					pilotAssign2 = pnj.Assignments[0]
					return true, pnj.ToJson()
				}
				return false, pnj.ToJson()
			}, 30*1000, 1000)
			assert.Equal(t, true, waitSucc, "应该能在超时前 pilotNode update, 耗时=%dms", elapsedMs)
			// step 7: simulate eph node update
			slog.InfoContext(ctx, "simulate eph node update",
				slog.String("event", "Step7"))
			setup.UpdateEphNode(workerFullId2, func(wej *cougarjson.WorkerEphJson) *cougarjson.WorkerEphJson {
				assign := cougarjson.NewAssignmentJson(pilotAssign2.ShardId, pilotAssign2.ReplicaIdx, pilotAssign2.AssignmentId, cougarjson.CAS_Ready)
				wej.Assignments = append(wej.Assignments, assign)
				wej.LastUpdateAtMs = kcommon.GetWallTimeMs()
				wej.LastUpdateReason = "simulate eph node update"
				return wej
			})
		}
		{
			// step 8: wail until routing node update
			slog.InfoContext(ctx, "wail until routing node update",
				slog.String("event", "Step8"))
			waitSucc, elapsedMs := setup.WaitUntilRoutingState(t, workerFullId, func(rj *unicornjson.WorkerEntryJson) (bool, string) {
				if rj == nil {
					return true, "没有 routing 节点"
				}
				return false, rj.ToJson()
			}, 60*1000, 3000)
			assert.Equal(t, true, waitSucc, "应该能在超时前 routing 节点被删除, 耗时=%dms", elapsedMs)
		}
	}

	// 使用 FakeTimeProvider 和模拟的 EtcdProvider/EtcdStore 运行测试
	setup.RunWith(fn)
}
