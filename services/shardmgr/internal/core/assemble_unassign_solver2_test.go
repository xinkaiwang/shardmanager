package core

import (
	"log/slog"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/libs/cougar/cougarjson"
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
	slog.InfoContext(ctx, "",
		slog.String("event", "测试环境已配置"))

	fn := func() {
		// Step 1: 创建 shardPlan and set into etcd
		// 添加1个分片 (shard_1)
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
			slog.InfoContext(ctx, "simulate eph node update",
				slog.String("event", "Step4"))
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
		slog.InfoContext(ctx, "request shutdown worker_1",
			slog.String("event", "Step5"))
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
		slog.InfoContext(ctx, "等待30秒",
			slog.String("event", "Step6"))
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
