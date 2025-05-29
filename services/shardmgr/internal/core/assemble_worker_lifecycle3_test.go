package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/libs/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
)

func TestAssembleWorkerLifeCycle3(t *testing.T) {
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
		// Step 1: 创建 ServiceState
		klogging.Info(ctx).Log("Step1", "创建 ServiceState")
		ss := AssembleSsAll(ctx, "TestAssembleAssignSolver")
		setup.ServiceState = ss
		klogging.Info(ctx).Log("ServiceState已创建", ss.Name)

		// Step 2: 创建 worker-1 eph
		klogging.Info(ctx).Log("Step2", "创建 worker-1 eph")
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

		// Step 5:
		klogging.Info(ctx).Log("Step5", "设置 replica count=2")
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
		klogging.Info(ctx).Log("Step6", "创建 worker-2 eph")
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
			klogging.Info(ctx).Log("Step7", "simulate eph node update")
			setup.UpdateEphNode(workerFullId2, func(wej *cougarjson.WorkerEphJson) *cougarjson.WorkerEphJson {
				assign := cougarjson.NewAssignmentJson(pilot2Assign.ShardId, pilot2Assign.ReplicaIdx, pilot2Assign.AsginmentId, cougarjson.CAS_Ready)
				wej.Assignments = append(wej.Assignments, assign)
				wej.LastUpdateAtMs = kcommon.GetWallTimeMs()
				wej.LastUpdateReason = "simulate eph node update"
				return wej
			})
		}

		// Step 8:
		klogging.Info(ctx).Log("Step8", "HardScore=0")
		{
			waitSucc, elapsedMs := setup.WaitUntilSnapshotCurrent(t, func(snap *costfunc.Snapshot) (bool, string) {
				if snap == nil {
					return false, "快照不存在"
				}
				cost := snap.GetCost()
				if cost.HardScore > 0 {
					return false, "快照不正确, cost=" + cost.String()
				}
				return true, "cost=" + cost.String()
			}, 10*1000, 1000)
			assert.Equal(t, true, waitSucc, "应该能在超时前快照, 耗时=%dms", elapsedMs)
		}

		// step 9: worker-1 request shutdown
		klogging.Info(ctx).Log("Step9", "request shutdown worker-1")
		{
			setup.UpdateEphNode(workerFullId, func(wej *cougarjson.WorkerEphJson) *cougarjson.WorkerEphJson {
				wej.ReqShutDown = 1
				wej.LastUpdateAtMs = setup.FakeTime.WallTime
				wej.LastUpdateReason = "SimulateDropShard"
				return wej
			})
		}

		// Step 10: hardScore=1
		klogging.Info(ctx).Log("Step10", "HardScore=1")
		{
			waitSucc, elapsedMs := setup.WaitUntilSnapshotCurrent(t, func(snap *costfunc.Snapshot) (bool, string) {
				if snap == nil {
					return false, "快照不存在"
				}
				cost := snap.GetCost()
				if cost.HardScore != 1 {
					return false, "快照不正确, cost=" + cost.String()
				}
				return true, "cost=" + cost.String()
			}, 10*1000, 1000)
			assert.Equal(t, true, waitSucc, "应该能在超时前快照, 耗时=%dms", elapsedMs)
		}

		// step 11: wait for long time
		klogging.Info(ctx).Log("Step11", "wait for long time")
		setup.FakeTime.VirtualTimeForward(ctx, 30*1000)

		// step 12: 1 replica
		klogging.Info(ctx).Log("Step12", "设置 replica count=1")
		setup.UpdateServiceConfig(config.WithShardConfig(func(sc *config.ShardConfig) {
			sc.MinReplicaCount = 1
			sc.MaxReplicaCount = 1
		}))

		// step 13: worker-1 should be unassigned (since it is shutdown requesting, so solver should favor worker-1 compare to worker-2)
		klogging.Info(ctx).Log("Step13", "worker-1 should be unassigned")
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
			klogging.Info(ctx).Log("Step14", "simulate eph node update")
			setup.UpdateEphNode(workerFullId, func(wej *cougarjson.WorkerEphJson) *cougarjson.WorkerEphJson {
				wej.Assignments = nil
				wej.LastUpdateAtMs = kcommon.GetWallTimeMs()
				wej.LastUpdateReason = "simulate eph node update"
				return wej
			})
		}

		// step 15: wait for long time
		klogging.Info(ctx).Log("Step15", "wait for long time")
		setup.FakeTime.VirtualTimeForward(ctx, 30*1000)

		// step 16: worker1 should get shutdown permit
		klogging.Info(ctx).Log("Step16", "worker-1 should get shutdown permit")
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
		klogging.Info(ctx).Log("Step17", "wait for long time")
		setup.FakeTime.VirtualTimeForward(ctx, 30*1000)

		// force print log
		// assert.Equal(t, true, false, "force print log")
	}

	// 使用 FakeTimeProvider 和模拟的 EtcdProvider/EtcdStore 运行测试
	setup.RunWith(fn)
}
