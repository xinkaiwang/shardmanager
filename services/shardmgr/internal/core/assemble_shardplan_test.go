package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/libs/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/libs/unicorn/unicornjson"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
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
	klogging.Info(ctx).Log("测试环境已配置", "")

	fn := func() {
		// Step 1: 创建 ServiceState
		klogging.Info(ctx).Log("Step1", "创建 ServiceState")
		ss := AssembleSsAll(ctx, "TestAssembleAssignSolver")
		setup.ServiceState = ss
		klogging.Info(ctx).Log("ServiceState已创建", ss.Name)

		// Step 2: 创建 worker-1 eph
		klogging.Info(ctx).Log("Step2", "创建 worker-1 eph")
		workerFullId, _ := setup.CreateAndSetWorkerEph(t, "worker-1", "session-1", "localhost:8080")

		klogging.Info(ctx).Log("Step3", "创建 shardPlan")
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
			klogging.Info(ctx).Log("Step4", "simulate eph node update")
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
		klogging.Info(ctx).Log("Step5", "Remove shardPlan")
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
			klogging.Info(ctx).Log("Step6", "simulate eph node update")
			setup.UpdateEphNode(workerFullId, func(wej *cougarjson.WorkerEphJson) *cougarjson.WorkerEphJson {
				wej.Assignments = nil
				wej.LastUpdateAtMs = kcommon.GetWallTimeMs()
				wej.LastUpdateReason = "simulate eph node update"
				return wej
			})
		}

		// Step 7:
		{
			klogging.Info(ctx).Log("Step7", "wait until no more moves")
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
		klogging.Info(ctx).Log("Step8", "wait for 60s")
		setup.FakeTime.VirtualTimeForward(ctx, 60*1000)
	}

	// 使用 FakeTimeProvider 和模拟的 EtcdProvider/EtcdStore 运行测试
	setup.RunWith(fn)
}
