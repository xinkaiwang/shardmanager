package core

import (
	"log/slog"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/libs/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
)

func TestAssembleWorkerLifeCycle3(t *testing.T) {
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
		{
			waitSucc, elapsedMs := setup.WaitUntilPilotNode(t, workerFullId, func(pnj *cougarjson.PilotNodeJson) (bool, string) {
				if pnj == nil {
					return false, "没有 pilot 节点"
				}
				return true, pnj.ToJson()
			}, 30*1000, 1000)
			assert.Equal(t, true, waitSucc, "应该能在超时前 pilotNode update, 耗时=%dms", elapsedMs)
		}

		// Step 3: 创建 shardPlan and set into etcd
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

		// Step 5:
		slog.InfoContext(ctx, "设置 replica count=2",
			slog.String("event", "Step5"))
		setup.UpdateServiceConfig(config.WithShardConfig(func(sc *config.ShardConfig) {
			sc.MinReplicaCount = 2
			sc.MaxReplicaCount = 2
		}))

		setup.FakeTime.VirtualTimeForward(ctx, 10*1000)

		{
			acceptCount := 0
			setup.safeAccessServiceState(func(ss *ServiceState) {
				acceptCount = ss.AcceptedCount
			})
			assert.Equal(t, 1, acceptCount, "should not have more than 1 accept count", acceptCount)
		}

		// Step 6: 创建 worker-2 eph
		slog.InfoContext(ctx, "创建 worker-2 eph",
			slog.String("event", "Step6"))
		workerFullId2, _ := setup.CreateAndSetWorkerEph(t, "worker-2", "session-2", "localhost:8080")
		{
			var pilot2Assign *cougarjson.PilotAssignmentJson
			waitSucc, elapsedMs := setup.WaitUntilPilotNode(t, workerFullId2, func(pnj *cougarjson.PilotNodeJson) (bool, string) {
				if pnj == nil {
					return false, "没有 pilot 节点"
				}
				if len(pnj.Assignments) == 1 {
					pilot2Assign = pnj.Assignments[0]
					return true, pnj.ToJson()
				}
				return false, pnj.ToJson()
			}, 30*1000, 1000)
			assert.Equal(t, true, waitSucc, "应该能在超时前 pilotNode update, 耗时=%dms", elapsedMs)
			// step7: simulate eph node update
			slog.InfoContext(ctx, "simulate eph node update",
				slog.String("event", "Step7"))
			setup.UpdateEphNode(workerFullId2, func(wej *cougarjson.WorkerEphJson) *cougarjson.WorkerEphJson {
				assign := cougarjson.NewAssignmentJson(pilot2Assign.ShardId, pilot2Assign.ReplicaIdx, pilot2Assign.AssignmentId, cougarjson.CAS_Ready)
				wej.Assignments = append(wej.Assignments, assign)
				wej.LastUpdateAtMs = kcommon.GetWallTimeMs()
				wej.LastUpdateReason = "simulate eph node update"
				return wej
			})
		}

		// Step 8:
		slog.InfoContext(ctx, "HardScore=0",
			slog.String("event", "Step8"))
		{
			waitSucc, elapsedMs := setup.WaitUntilSnapshotCurrent(t, func(snap *costfunc.Snapshot) (bool, string) {
				if snap == nil {
					return false, "快照不存在"
				}
				cost := snap.GetCost(ctx)
				if cost.HardScore > 0 {
					return false, "快照不正确, cost=" + cost.String()
				}
				return true, "cost=" + cost.String()
			}, 10*1000, 1000)
			assert.Equal(t, true, waitSucc, "应该能在超时前快照, 耗时=%dms", elapsedMs)
		}

		// step 9: worker-1 request shutdown
		slog.InfoContext(ctx, "request shutdown worker-1",
			slog.String("event", "Step9"))
		{
			setup.UpdateEphNode(workerFullId, func(wej *cougarjson.WorkerEphJson) *cougarjson.WorkerEphJson {
				wej.ReqShutDown = 1
				wej.LastUpdateAtMs = setup.FakeTime.WallTime
				wej.LastUpdateReason = "SimulateDropShard"
				return wej
			})
		}

		// Step 10: hardScore=1
		slog.InfoContext(ctx, "HardScore=1",
			slog.String("event", "Step10"))
		{
			waitSucc, elapsedMs := setup.WaitUntilSnapshotCurrent(t, func(snap *costfunc.Snapshot) (bool, string) {
				if snap == nil {
					return false, "快照不存在"
				}
				cost := snap.GetCost(ctx)
				if cost.HardScore != 1 {
					return false, "快照不正确, cost=" + cost.String()
				}
				return true, "cost=" + cost.String()
			}, 10*1000, 1000)
			assert.Equal(t, true, waitSucc, "应该能在超时前快照, 耗时=%dms", elapsedMs)
		}

		// step 11: wait for long time
		slog.InfoContext(ctx, "wait for long time",
			slog.String("event", "Step11"))
		setup.FakeTime.VirtualTimeForward(ctx, 30*1000)

		// step 12: 1 replica
		slog.InfoContext(ctx, "设置 replica count=1",
			slog.String("event", "Step12"))
		setup.UpdateServiceConfig(config.WithShardConfig(func(sc *config.ShardConfig) {
			sc.MinReplicaCount = 1
			sc.MaxReplicaCount = 1
		}))

		// step 13: worker-1 should be unassigned (since it is shutdown requesting, so solver should favor worker-1 compare to worker-2)
		slog.InfoContext(ctx, "worker-1 should be unassigned",
			slog.String("event", "Step13"))
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
			// step14: simulate eph node update
			slog.InfoContext(ctx, "simulate eph node update",
				slog.String("event", "Step14"))
			setup.UpdateEphNode(workerFullId, func(wej *cougarjson.WorkerEphJson) *cougarjson.WorkerEphJson {
				wej.Assignments = nil
				wej.LastUpdateAtMs = kcommon.GetWallTimeMs()
				wej.LastUpdateReason = "simulate eph node update"
				return wej
			})
		}

		// step 15: wait for long time
		slog.InfoContext(ctx, "wait for long time",
			slog.String("event", "Step15"))
		setup.FakeTime.VirtualTimeForward(ctx, 30*1000)

		// step 16: worker1 should get shutdown permit
		slog.InfoContext(ctx, "worker-1 should get shutdown permit",
			slog.String("event", "Step16"))
		{
			waitSucc, elapsedMs := setup.WaitUntilPilotNode(t, workerFullId, func(pnj *cougarjson.PilotNodeJson) (bool, string) {
				if pnj == nil {
					return false, "没有 pilot 节点"
				}
				if pnj.ShutdownPermited == 1 {
					return true, pnj.ToJson() // 这里需要注意，ShutdownPermited 是 int8 类型，所以要用 == 1 来判断
				}
				return false, "pilot 节点没有 ShutdownPermited"
			}, 30*1000, 1000)
			assert.Equal(t, true, waitSucc, "应该能在超时前 pilotNode update, 耗时=%dms", elapsedMs)
		}

		// step 17: wait for long time
		slog.InfoContext(ctx, "wait for long time",
			slog.String("event", "Step17"))
		setup.FakeTime.VirtualTimeForward(ctx, 30*1000)

		// force print log
		// assert.Equal(t, true, false, "force print log")
	}

	// 使用 FakeTimeProvider 和模拟的 EtcdProvider/EtcdStore 运行测试
	setup.RunWith(fn)
}
