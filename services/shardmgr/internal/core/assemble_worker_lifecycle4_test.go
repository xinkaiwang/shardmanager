package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/unicorn/unicornjson"
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
				assign := cougarjson.NewAssignmentJson(pilotAssign.ShardId, pilotAssign.ReplicaIdx, pilotAssign.AsginmentId, cougarjson.CAS_Ready)
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
		klogging.Info(ctx).Log("Step5", "delete worker eph 节点")
		setup.UpdateEphNode(workerFullId, func(wej *cougarjson.WorkerEphJson) *cougarjson.WorkerEphJson {
			return nil
		})

		klogging.Info(ctx).Log("Step6", "创建 worker-2 eph")
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
			setup.UpdateEphNode(workerFullId2, func(wej *cougarjson.WorkerEphJson) *cougarjson.WorkerEphJson {
				assign := cougarjson.NewAssignmentJson(pilotAssign2.ShardId, pilotAssign2.ReplicaIdx, pilotAssign2.AsginmentId, cougarjson.CAS_Ready)
				wej.Assignments = append(wej.Assignments, assign)
				wej.LastUpdateAtMs = kcommon.GetWallTimeMs()
				wej.LastUpdateReason = "simulate eph node update"
				return wej
			})
		}
		{
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
