package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/libs/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

/*
Step 1: 创建 shardPlan -> write into (fake) etcd
Step 2: 创建 worker-1 eph -> write to (fake) etcd
Step 3: 创建 ServiceState
Step 4: 等待 ServiceState 加载分片状态
Step 5: 等待 worker state 创建
Step 6: simulate a solver move -> Enqueue proposal
Step 7: 等待 accept (接受提案)
Step 8: 等待 worker pilot node 更新
Step 9: simulate worker eph -> write into (fake) etcd
Step 10: 等待 worker eph sync to worker state
Step 11: done
*/
func TestAssembleFakeSolver2(t *testing.T) {
	ctx := context.Background()
	// klogging.SetDefaultLogger(klogging.NewLogrusLogger(ctx).SetConfig(ctx, "debug", "text"))

	// 配置测试环境
	setup := NewFakeTimeTestSetup(t)
	setup.SetupBasicConfig(ctx)
	t.Logf("测试环境已配置")

	fn := func() {
		// Step 1: 创建 worker-1 eph
		klogging.Info(ctx).Log("Step1", "创建 worker-1 eph")
		workerFullId, _ := setup.CreateAndSetWorkerEph(t, "worker-1", "session-1", "localhost:8080")

		// Step 2: 创建 shardPlan and set into etcd
		// 添加三个分片 (shard-a, shard-b, shard-c)
		klogging.Info(ctx).Log("Step2", "创建 shardPlan and set into etcd")
		setup.FakeTime.VirtualTimeForward(ctx, 10)
		firstShardPlan := []string{"shard-a", "shard-b", "shard-c"}
		setShardPlan(t, setup.FakeEtcd, ctx, firstShardPlan)

		// Step 3: 创建 ServiceState
		klogging.Info(ctx).Log("Step3", "创建 ServiceState")
		ss := AssembleSsWithShadowState(ctx, "TestAssembleFakeSolver")
		ss.SolverGroup = setup.FakeSnapshotListener
		ss.SolverGroup.OnSnapshot(ctx, ss.GetSnapshotFutureForClone(ctx), "TestAssembleFakeSolver")
		setup.ServiceState = ss
		t.Logf("ServiceState已创建: %s", ss.Name)

		{
			// 等待worker state创建
			waitSucc, elapsedMs := setup.WaitUntilWorkerStateEnum(t, workerFullId, data.WS_Online_healthy, 1000, 10)
			assert.Equal(t, true, waitSucc, "worker state 已创建 elapsedMs=%d", elapsedMs)
		}

		// 等待ServiceState加载分片状态
		{
			// 等待快照更新
			waitSucc, elapsedMs := setup.WaitUntilSnapshot(t, func(snapshot *costfunc.Snapshot) (bool, string) {
				if snapshot == nil {
					return false, "快照未创建"
				}
				if snapshot.GetCost().HardScore == 6 {
					return true, ""
				}
				return false, "快照更新未达预期:" + snapshot.GetCost().String()
			}, 1000, 10)
			assert.True(t, waitSucc, "应该能在超时前加载所有分片, 耗时=%dms", elapsedMs)
		}
		{
			// assert.Equal(t, 3, snapshotListener.updateCount, "快照更新次数应该为3") // 1:first create (empty), 2: worker added, 3: shards added
			assert.NotNil(t, setup.FakeSnapshotListener.snapshot, "快照应该不为nil")
			cost := setup.FakeSnapshotListener.snapshot.GetCost()
			assert.Equal(t, costfunc.Cost{HardScore: 6}, cost, "快照的成本应该为0")
		}

		// Step 4: simulate a solver move
		klogging.Info(ctx).Log("Step4", "simulate a solver move")
		{
			shardId := data.ShardId("shard-a")
			replicaFullId := data.NewReplicaFullId(shardId, 0)
			move := costfunc.NewAssignMove(replicaFullId, data.AssignmentId("as1"), workerFullId)
			proposal := costfunc.NewProposal(ctx, "TestAssembleFakeSolver", costfunc.Gain{HardScore: 1}, setup.FakeSnapshotListener.snapshot.SnapshotId)
			proposal.Move = move
			ss.ProposalQueue.Push(proposal)
		}

		// Step 5: 等待accept (接受提案)
		klogging.Info(ctx).Log("Step5", "等待accept (接受提案)")
		{
			// 等待提案接受
			waitSucc, elapsedMs := WaitUntil(t, func() (bool, string) {
				if ss.AcceptedCount == 1 {
					return true, "accept数目正确"
				}
				return false, "accept数目未达预期"
			}, 1000, 100)
			assert.True(t, waitSucc, "应该能在超时前接受提案, 耗时=%dms", elapsedMs)
		}
		{
			// 等待 move
			waitSucc, elapsedMs := WaitUntil(t, func() (bool, string) {
				if len(ss.AllMoves) == 1 {
					return true, "in-flight move 数目正确"
				}
				return false, "in-flight move 数目未达预期"
			}, 1000, 10)
			assert.True(t, waitSucc, "应该能在超时前 move, 耗时=%dms", elapsedMs)
		}
		{
			// 等待 pilot node
			waitSucc, elapsedMs := WaitUntil(t, func() (bool, string) {
				pilotNode := setup.GetPilotNode(workerFullId)
				if pilotNode == nil {
					return false, "pilot node 不存在"
				}
				if len(pilotNode.Assignments) == 1 {
					return true, "pilot assignment 数目正确"
				}
				return false, "assignment 数目未达预期"
			}, 1000, 10)
			assert.True(t, waitSucc, "应该能在超时前 update pilotNode, 耗时=%dms", elapsedMs)
		}

		// Step 6: Fake worker eph
		klogging.Info(ctx).Log("Step6", "Fake worker eph")
		{
			ephNode := setup.GetEphNode(workerFullId)
			assign := cougarjson.NewAssignmentJson("shard-a", 0, "as1", cougarjson.CAS_Ready)
			ephNode.Assignments = append(ephNode.Assignments, assign)
			ephNode.LastUpdateAtMs = kcommon.GetWallTimeMs()
			ephNode.LastUpdateReason = "AddShard"
			// 设置到etcd
			ephPath := ss.PathManager.FmtWorkerEphPath(workerFullId)
			setup.FakeEtcd.Set(setup.ctx, ephPath, ephNode.ToJson())
		}

		{
			// 等待 worker eph sync to worker state
			waitSucc, elapsedMs := setup.WaitUntilWorkerFullState(t, workerFullId, func(ws *WorkerState, dict map[data.AssignmentId]*AssignmentState) (bool, string) {
				if ws == nil {
					return false, "worker state 不存在"
				}
				if len(ws.Assignments) == 1 {
					for _, assignId := range ws.Assignments {
						assignState := dict[assignId]
						if assignState.CurrentConfirmedState == cougarjson.CAS_Ready {
							return true, ""
						}
						return false, "assignment 状态不正确:" + string(assignState.CurrentConfirmedState)
					}
				}
				return false, "assignment 数目未达预期"
			}, 1000, 10)
			assert.True(t, waitSucc, "应该能在超时前 update worker eph, 耗时=%dms", elapsedMs)
		}

		// stop
		ss.StopAndWaitForExit(ctx)
		setup.PrintAll(ctx)

		// assert.Equal(t, true, false, "") // 强制查看测试输出
	}
	// 使用 FakeTimeProvider 和模拟的 EtcdProvider/EtcdStore 运行测试
	setup.RunWith(fn)
}
