package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

func TestAssembleAssignSolver(t *testing.T) {
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
		// 添加1个分片 (shard_1, )
		firstShardPlan := []string{"shard_1"}
		setup.SetShardPlan(ctx, firstShardPlan)

		// Step 2: 创建 ServiceState
		ss := AssembleSsAll(ctx, "TestAssembleAssignSolver")
		setup.ServiceState = ss
		klogging.Info(ctx).Log("ServiceState已创建", ss.Name)

		{
			// 等待快照更新
			waitSucc, elapsedMs := setup.WaitUntilSs(t, func(ss *ServiceState) (bool, string) {
				if ss.SnapshotFuture == nil {
					return false, "快照不存在"
				}
				if !ss.SnapshotFuture.GetCost().IsEqualTo(costfunc.NewCost(2, 0.0)) {
					return false, "快照不正确" + ss.SnapshotFuture.GetCost().String()
				}
				return true, "" // 快照存在
			}, 1000, 10)
			assert.True(t, waitSucc, "应该能在超时前创建快照, 耗时=%dms", elapsedMs)
		}
		// Step 3: 创建 worker-1 eph
		workerFullId, _ := setup.CreateAndSetWorkerEph(t, "worker-1", "session-1", "localhost:8080")

		{
			// 等待worker state创建
			waitSucc, elapsedMs := setup.WaitUntilWorkerStateEnum(t, workerFullId, data.WS_Online_healthy, 1000, 10)
			assert.Equal(t, true, waitSucc, "worker state 已创建 elapsedMs=%d", elapsedMs)
		}

		// Step 4: Enable AssignSolver
		setup.UpdateServiceConfig(config.WithSolverConfig(func(sc *config.SolverConfig) {
			sc.AssignSolverConfig.SolverEnabled = true
		}))

		{
			// 等待接受提议
			waitSucc, elapsedMs := setup.WaitUntilSs(t, func(ss *ServiceState) (bool, string) {
				if ss.AcceptedCount == 0 {
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
				if ss.SnapshotFuture == nil {
					reason = "快照不存在"
				}
				cost := ss.SnapshotFuture.GetCost()
				if cost.HardScore > 0 {
					reason = "快照不正确, cost=" + cost.String()
				}
				ok = true
			})
			assert.True(t, ok, reason)
		}

		var pilotAssign *cougarjson.PilotAssignmentJson
		{
			waitSucc, elapsedMs := setup.WaitUntilPilotNode(t, workerFullId, func(pnj *cougarjson.PilotNodeJson) (bool, string) {
				if pnj == nil {
					return false, "没有 pilot 节点"
				}
				if len(pnj.Assignments) == 0 {
					return false, "没有 assignment"
				}
				if pnj.Assignments[0].ShardId != "shard_1" {
					return false, "assignment 不正确"
				}
				pilotAssign = pnj.Assignments[0]
				return true, ""
			}, 1*1000, 100)
			assert.Equal(t, true, waitSucc, "应该能在超时前 pilotNode update, 耗时=%dms", elapsedMs)
		}

		// Step 5: simulate eph node update
		setup.UpdateEphNode(workerFullId, func(wej *cougarjson.WorkerEphJson) *cougarjson.WorkerEphJson {
			wej.Assignments = append(wej.Assignments, cougarjson.NewAssignmentJson(pilotAssign.ShardId, pilotAssign.ReplicaIdx, pilotAssign.AsginmentId, cougarjson.CAS_Ready))
			wej.LastUpdateAtMs = setup.FakeTime.WallTime
			wej.LastUpdateReason = "SimulateAddShard"
			return wej
		})

		{
			waitSucc, elapsedMs := setup.WaitUntilWorkerFullState(t, workerFullId, func(ws *WorkerState, assigns map[data.AssignmentId]*AssignmentState) (bool, string) {
				if ws == nil {
					return false, "没有 worker 节点"
				}
				if len(ws.Assignments) == 0 {
					return false, "没有 assignment"
				}
				if assign, ok := assigns[data.AssignmentId(pilotAssign.AsginmentId)]; !ok {
					return false, "没有 assignment"
				} else {
					if assign.CurrentConfirmedState != cougarjson.CAS_Ready {
						return false, "assignment 状态不正确:" + string(assign.CurrentConfirmedState)
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
				if ss.SnapshotCurrent == nil {
					reason = "快照不存在"
				}
				cost := ss.SnapshotCurrent.GetCost()
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
