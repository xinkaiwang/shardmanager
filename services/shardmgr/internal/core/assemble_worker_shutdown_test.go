package core

import (
	"log/slog"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/libs/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
)

func TestAssembleWorker_shutdown(t *testing.T) {
	ctx := context.Background()

	// 配置测试环境
	setup := NewFakeTimeTestSetup(t)
	ctx = setup.ctx
	setup.SetupBasicConfig(ctx, config.WithSolverConfig(func(sc *config.SolverConfig) {
		sc.SoftSolverConfig.SolverEnabled = true
		sc.AssignSolverConfig.SolverEnabled = true
		sc.UnassignSolverConfig.SolverEnabled = true
	}))
	slog.InfoContext(ctx, "",
		slog.String("event", "测试环境已配置"))

	fn := func() {
		// Step 1: 创建 shardPlan and set into etcd
		// 添加2个分片 (shard_1/2)
		slog.InfoContext(ctx, "创建 shardPlan",
			slog.String("event", "Step1"))
		firstShardPlan := []string{"shard_1"}
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
		workerFullId, _ := setup.CreateAndSetWorkerEph(t, "worker-1", "session-1", "localhost:8081")

		setup.FakeTime.VirtualTimeForward(ctx, 60*1000)
		{
			// step 4: wait for pilot
			slog.InfoContext(ctx, "wait for pilot",
				slog.String("event", "Step4"))
			var pilotAssign *cougarjson.PilotAssignmentJson
			waitSucc, elapsedMs := setup.WaitUntilPilotNode(t, workerFullId, func(pnj *cougarjson.PilotNodeJson) (bool, string) {
				if pnj == nil {
					return false, "没有 pilot 节点"
				}
				if len(pnj.Assignments) == 0 {
					return false, "polot 没有 assignment"
				}
				if pnj.Assignments[0].ShardId != "shard_1" {
					return false, "pilot assignment 不正确"
				}
				pilotAssign = pnj.Assignments[0]
				return true, pilotAssign.ToJson()
			}, 30*1000, 1000)
			assert.Equal(t, true, waitSucc, "应该能在超时前 pilotNode update, 耗时=%dms", elapsedMs)

			// Step 5: simulate eph node update
			slog.InfoContext(ctx, "simulate eph node update",
				slog.String("event", "Step5"))
			setup.UpdateEphNode(workerFullId, func(wej *cougarjson.WorkerEphJson) *cougarjson.WorkerEphJson {
				wej.Assignments = append(wej.Assignments, cougarjson.NewAssignmentJson(pilotAssign.ShardId, pilotAssign.ReplicaIdx, pilotAssign.AssignmentId, cougarjson.CAS_Ready))
				wej.LastUpdateAtMs = setup.FakeTime.WallTime
				wej.LastUpdateReason = "SimulateAddShard"
				return wej
			})
		}

		// Step 6: 创建 worker-2 eph
		slog.InfoContext(ctx, "创建 worker-2 eph",
			slog.String("event", "Step6"))
		setup.SetShardPlan(ctx, []string{"shard_1", "shard_2"})
		workerFullId2, _ := setup.CreateAndSetWorkerEph(t, "worker-2", "session-2", "localhost:8082")

		{
			// step 6: wait for pilot
			var pilotAssign *cougarjson.PilotAssignmentJson
			waitSucc, elapsedMs := setup.WaitUntilPilotNode(t, workerFullId2, func(pnj *cougarjson.PilotNodeJson) (bool, string) {
				if pnj == nil {
					return false, "没有 pilot 节点"
				}
				if len(pnj.Assignments) == 0 {
					return false, "polot 没有 assignment"
				}
				if pnj.Assignments[0].ShardId != "shard_2" {
					return false, "pilot assignment 不正确"
				}
				pilotAssign = pnj.Assignments[0]
				return true, pilotAssign.ToJson()
			}, 30*1000, 1000)
			assert.Equal(t, true, waitSucc, "应该能在超时前 pilotNode update, 耗时=%dms", elapsedMs)
			// Step 7: simulate eph node update
			slog.InfoContext(ctx, "simulate eph node update",
				slog.String("event", "Step7"))
			setup.UpdateEphNode(workerFullId2, func(wej *cougarjson.WorkerEphJson) *cougarjson.WorkerEphJson {
				wej.Assignments = append(wej.Assignments, cougarjson.NewAssignmentJson(pilotAssign.ShardId, pilotAssign.ReplicaIdx, pilotAssign.AssignmentId, cougarjson.CAS_Ready))
				wej.LastUpdateAtMs = setup.FakeTime.WallTime
				wej.LastUpdateReason = "SimulateAddShard"
				return wej
			})
		}

		// step 8: worker 1 shutdown request
		slog.InfoContext(ctx, "worker 1 shutdown request",
			slog.String("event", "Step8"))
		setup.UpdateEphNode(workerFullId, func(wej *cougarjson.WorkerEphJson) *cougarjson.WorkerEphJson {
			wej.ReqShutDown = 1
			wej.LastUpdateAtMs = kcommon.GetWallTimeMs()
			wej.LastUpdateReason = "SimulateDropShard"
			return wej
		})

		// Step 9: 等待 pilot 节点更新
		{
			var pilotAssigns []*cougarjson.PilotAssignmentJson
			waitSucc, elapsedMs := setup.WaitUntilPilotNode(t, workerFullId2, func(pnj *cougarjson.PilotNodeJson) (bool, string) {
				if pnj == nil {
					return false, "没有 pilot 节点"
				}
				if len(pnj.Assignments) == 2 {
					pilotAssigns = pnj.Assignments
					return true, "assignment added"
				}
				return false, "assignment not removed"
			}, 30*1000, 1000)
			assert.Equal(t, true, waitSucc, "应该能在超时前 pilotNode update, 耗时=%dms", elapsedMs)
			// Step 10: simulate eph node update
			slog.InfoContext(ctx, "simulate eph node update",
				slog.String("event", "Step10"))
			setup.UpdateEphNode(workerFullId2, func(wej *cougarjson.WorkerEphJson) *cougarjson.WorkerEphJson {
				wej.Assignments = nil
				for _, pilotAssign := range pilotAssigns {
					wej.Assignments = append(wej.Assignments, cougarjson.NewAssignmentJson(pilotAssign.ShardId, pilotAssign.ReplicaIdx, pilotAssign.AssignmentId, cougarjson.CAS_Ready))
				}
				wej.LastUpdateAtMs = setup.FakeTime.WallTime
				wej.LastUpdateReason = "SimulateAddShard"
				return wej
			})
		}
		{
			// step 11: wait for pilot
			slog.InfoContext(ctx, "wait for pilot",
				slog.String("event", "Step11"))
			waitSucc, elapsedMs := setup.WaitUntilPilotNode(t, workerFullId, func(pnj *cougarjson.PilotNodeJson) (bool, string) {
				if pnj == nil {
					return false, "没有 pilot 节点"
				}
				if len(pnj.Assignments) == 0 {
					return true, "assignment removed"
				}
				return false, "assignment not removed"
			}, 30*1000, 1000)
			assert.Equal(t, true, waitSucc, "应该能在超时前 pilotNode update, 耗时=%dms", elapsedMs)
		}
	}

	// 使用 FakeTimeProvider 和模拟的 EtcdProvider/EtcdStore 运行测试
	setup.RunWith(fn)

}
