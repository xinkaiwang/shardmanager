package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/libs/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
)

func TestAssembleUnassignSolver2(t *testing.T) {
	ctx := context.Background()

	// 配置测试环境
	setup := NewFakeTimeTestSetup(t)
	setup.SetupBasicConfig(ctx, config.WithSolverConfig(func(sc *config.SolverConfig) {
		sc.SoftSolverConfig.SolverEnabled = false
		sc.AssignSolverConfig.SolverEnabled = true
		sc.UnassignSolverConfig.SolverEnabled = true
	}))
	klogging.Info(ctx).Log("测试环境已配置", "")

	fn := func() {
		// Step 1: 创建 shardPlan and set into etcd
		// 添加1个分片 (shard_1)
		klogging.Info(ctx).Log("Step1", "创建 shardPlan")
		firstShardPlan := []string{"shard_1"}
		setup.SetShardPlan(ctx, firstShardPlan)

		// Step 2: 创建 ServiceState
		klogging.Info(ctx).Log("Step2", "创建 ServiceState")
		ss := AssembleSsAll(ctx, "TestAssembleAssignSolver")
		setup.ServiceState = ss
		klogging.Info(ctx).Log("ServiceState已创建", ss.Name)

		// Step 3: 创建 worker-1 eph
		klogging.Info(ctx).Log("Step3", "创建 worker-1 eph")
		workerFullId, _ := setup.CreateAndSetWorkerEph(t, "worker-1", "session-1", "localhost:8080")

		{
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

			// Step 4: simulate eph node update
			klogging.Info(ctx).Log("Step4", "simulate eph node update")
			setup.UpdateEphNode(workerFullId, func(wej *cougarjson.WorkerEphJson) *cougarjson.WorkerEphJson {
				wej.Assignments = append(wej.Assignments, cougarjson.NewAssignmentJson(pilotAssign.ShardId, pilotAssign.ReplicaIdx, pilotAssign.AssignmentId, cougarjson.CAS_Ready))
				wej.LastUpdateAtMs = setup.FakeTime.WallTime
				wej.LastUpdateReason = "SimulateAddShard"
				return wej
			})
		}

		{
			// verify current snapshot
			// at this point, since we already completed the "assign" move, so we should have a new current snapshot (which is cost 0, all shards are assigned)
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
		{
			waitSucc, elapsedMs := setup.WaitUntilSs(t, func(ss *ServiceState) (bool, string) {
				if len(ss.AllMoves) > 0 {
					return false, "有未完成的 move"
				}
				return true, "move完成" // 没有未完成的 move
			}, 10*1000, 1000)
			assert.Equal(t, true, waitSucc, "应该能在超时前 routing 节点 update, 耗时=%dms", elapsedMs)
		}

		// Step 5: worker_1 request shutdown (unassign solver should be triggered)
		klogging.Info(ctx).Log("Step5", "request shutdown worker_1")
		setup.UpdateEphNode(workerFullId, func(wej *cougarjson.WorkerEphJson) *cougarjson.WorkerEphJson {
			wej.ReqShutDown = 1
			wej.LastUpdateAtMs = setup.FakeTime.WallTime
			wej.LastUpdateReason = "SimulateDropShard"
			return wej
		})

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

		// Step6: wait for 30 seconds
		klogging.Info(ctx).Log("Step6", "等待30秒")
		setup.FakeTime.VirtualTimeForward(ctx, 30*1000)

		{
			acceptCount := 0
			setup.safeAccessServiceState(func(ss *ServiceState) {
				acceptCount = ss.AcceptedCount
			})
			assert.Equal(t, 1, acceptCount, "接受提议 = 1")
		}
	}

	// 使用 FakeTimeProvider 和模拟的 EtcdProvider/EtcdStore 运行测试
	setup.RunWith(fn)
}
