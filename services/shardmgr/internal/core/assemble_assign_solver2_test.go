package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/libs/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

func TestAssembleAssignSolver_2_shards(t *testing.T) {
	ctx := context.Background()

	// 配置测试环境
	setup := NewFakeTimeTestSetup(t)
	setup.SetupBasicConfig(ctx, config.WithSolverConfig(func(sc *config.SolverConfig) {
		sc.SoftSolverConfig.SolverEnabled = false
		sc.AssignSolverConfig.SolverEnabled = false
		sc.UnassignSolverConfig.SolverEnabled = false
	}))
	klogging.Info(ctx).Log("测试环境已配置", "")

	fn := func() {
		// Step 1: 创建 shardPlan and set into etcd
		// 添加2个分片
		klogging.Info(ctx).Log("Step1", "创建 shardPlan")
		firstShardPlan := []string{"shard_1", "shard_2"}
		setup.SetShardPlan(ctx, firstShardPlan)

		// Step 2: 创建 ServiceState
		klogging.Info(ctx).Log("Step2", "创建 ServiceState")
		ss := AssembleSsAll(ctx, "TestAssembleAssignSolver")
		setup.ServiceState = ss
		klogging.Info(ctx).Log("ServiceState已创建", ss.Name)

		{
			// 等待快照更新
			waitSucc, elapsedMs := setup.WaitUntilSs(t, func(ss *ServiceState) (bool, string) {
				if ss.GetSnapshotFutureForAny(ctx) == nil {
					return false, "快照不存在"
				}
				if !ss.GetSnapshotFutureForAny(ctx).GetCost(ctx).IsEqualTo(costfunc.NewCost(4, 0.0)) {
					return false, "快照不正确" + ss.GetSnapshotFutureForAny(ctx).GetCost(ctx).String()
				}
				return true, "" // 快照存在
			}, 1000, 10)
			assert.True(t, waitSucc, "应该能在超时前创建快照, 耗时=%dms", elapsedMs)
		}
		// Step 3: 创建 worker-1 eph
		klogging.Info(ctx).Log("Step3", "创建 worker-1 eph")
		workerFullId, _ := setup.CreateAndSetWorkerEph(t, "worker-1", "session-1", "localhost:8080")

		{
			// 等待worker state创建
			waitSucc, elapsedMs := setup.WaitUntilWorkerStateEnum(t, workerFullId, data.WS_Online_healthy, 1000, 100)
			assert.Equal(t, true, waitSucc, "worker state 已创建 elapsedMs=%d", elapsedMs)
		}

		// Step 4: Enable AssignSolver
		klogging.Info(ctx).Log("Step4", "Enable AssignSolver")
		setup.UpdateServiceConfig(config.WithSolverConfig(func(sc *config.SolverConfig) {
			sc.AssignSolverConfig.SolverEnabled = true
		}))

		{
			// 等待接受提议
			waitSucc, elapsedMs := setup.WaitUntilSs(t, func(ss *ServiceState) (bool, string) {
				if ss.AcceptedCount < 1 {
					return false, "没有接受的提议"
				}
				return true, "" // 有接受的提议
			}, 10*1000, 1000)
			assert.Equal(t, true, waitSucc, "应该能在超时前创建快照, 耗时=%dms", elapsedMs)
		}
		{
			// 等待接受提议
			waitSucc, elapsedMs := setup.WaitUntilSs(t, func(ss *ServiceState) (bool, string) {
				if ss.AcceptedCount < 2 {
					return false, "没有接受的提议"
				}
				return true, "" // 有接受的提议
			}, 10*1000, 1000)
			assert.Equal(t, true, waitSucc, "应该能在超时前创建快照, 耗时=%dms", elapsedMs)
		}
		{
			// verify future snapshot
			// at this point, since we already accept the "assign" proposal, so we should have a new future snapshot (which is cost 0, all shards are assigned)
			ok := false
			var reason string
			setup.safeAccessServiceState(func(ss *ServiceState) {
				if ss.GetSnapshotFutureForAny(ctx) == nil {
					reason = "快照不存在"
				}
				cost := ss.GetSnapshotFutureForAny(ctx).GetCost(ctx)
				if cost.HardScore > 0 {
					reason = "快照不正确, cost=" + cost.String()
				}
				ok = true
			})
			assert.True(t, ok, reason)
		}

		// step5: simulate pilot node update
		klogging.Info(ctx).Log("Step5", "simulate pilot node update")
		var pilotAssign1 *cougarjson.PilotAssignmentJson
		var pilotAssign2 *cougarjson.PilotAssignmentJson
		{
			waitSucc, elapsedMs := setup.WaitUntilPilotNode(t, workerFullId, func(pnj *cougarjson.PilotNodeJson) (bool, string) {
				if pnj == nil {
					return false, "没有 pilot 节点"
				}
				if len(pnj.Assignments) < 2 {
					return false, "没有 assignment"
				}
				pilotAssign1 = pnj.Assignments[0]
				pilotAssign2 = pnj.Assignments[1]
				return true, ""
			}, 1*1000, 100)
			assert.Equal(t, true, waitSucc, "应该能在超时前 pilotNode update, 耗时=%dms", elapsedMs)
		}

		// Step 5: simulate eph node update
		if pilotAssign1 != nil && pilotAssign2 != nil {
			setup.UpdateEphNode(workerFullId, func(wej *cougarjson.WorkerEphJson) *cougarjson.WorkerEphJson {
				wej.Assignments = append(wej.Assignments, cougarjson.NewAssignmentJson(pilotAssign1.ShardId, pilotAssign1.ReplicaIdx, pilotAssign1.AssignmentId, cougarjson.CAS_Ready))
				wej.Assignments = append(wej.Assignments, cougarjson.NewAssignmentJson(pilotAssign2.ShardId, pilotAssign2.ReplicaIdx, pilotAssign2.AssignmentId, cougarjson.CAS_Ready))
				wej.LastUpdateAtMs = setup.FakeTime.WallTime
				wej.LastUpdateReason = "SimulateAddShard"
				return wej
			})
		}

		{
			waitSucc, elapsedMs := setup.WaitUntilWorkerFullState(t, workerFullId, func(ws *WorkerState, assigns map[data.AssignmentId]*AssignmentState) (bool, string) {
				if ws == nil {
					return false, "没有 worker 节点"
				}
				if len(ws.Assignments) == 0 {
					return false, "没有 assignment"
				}
				{
					if assign, ok := assigns[data.AssignmentId(pilotAssign1.AssignmentId)]; !ok {
						return false, "没有 assignment"
					} else {
						if assign.CurrentConfirmedState != cougarjson.CAS_Ready {
							return false, "assignment 状态不正确:" + string(assign.CurrentConfirmedState)
						}
					}
				}
				{
					if assign, ok := assigns[data.AssignmentId(pilotAssign2.AssignmentId)]; !ok {
						return false, "没有 assignment"
					} else {
						if assign.CurrentConfirmedState != cougarjson.CAS_Ready {
							return false, "assignment 状态不正确:" + string(assign.CurrentConfirmedState)
						}
					}
				}
				return true, ""
			}, 1*1000, 100)
			assert.Equal(t, true, waitSucc, "应该能在超时前 workerFullState update, 耗时=%dms", elapsedMs)
		}

		{
			// verify current snapshot
			// at this point, since we already completed the "assign" move, so we should have a new current snapshot (which is cost 0, all shards are assigned)
			ok := false
			var reason string
			setup.safeAccessServiceState(func(ss *ServiceState) {
				if ss.GetSnapshotCurrentForAny() == nil {
					reason = "快照不存在"
				}
				cost := ss.GetSnapshotCurrentForAny().GetCost(ctx)
				if cost.HardScore > 0 {
					reason = "快照不正确, cost=" + cost.String()
				}
				ok = true
			})
			assert.True(t, ok, reason)
		}

		// assert.Equal(t, true, false, "") // 强制查看测试输出
	}

	// 使用 FakeTimeProvider 和模拟的 EtcdProvider/EtcdStore 运行测试
	setup.RunWith(fn)
}
