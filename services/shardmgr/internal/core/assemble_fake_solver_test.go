package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/libs/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/libs/unicorn/unicornjson"
)

// implements solver.SnapshotListener
type MySnapshotListener struct {
	snapshot    *costfunc.Snapshot
	updateCount int
}

func (l *MySnapshotListener) OnSnapshot(snapshot *costfunc.Snapshot) {
	l.snapshot = snapshot
	l.updateCount++
}

/*
Step 1: 创建 ServiceState
Step 2: 创建 worker-1 eph -> write to (fake) etcd
Step 3: 创建 shardPlan -> write into (fake) etcd
Step 4: 等待 ServiceState 加载分片状态
Step 5: 等待 worker state 创建
Step 6: simulate a solver move -> Enqueue proposal
Step 7: 等待 accept (接受提案)
Step 8: 等待 worker pilot node 更新
Step 9: simulate worker eph -> write into (fake) etcd
Step 10: 等待 worker eph sync to worker state
Step 11: done
*/
func TestAssembleFakeSolver(t *testing.T) {
	ctx := context.Background()
	// klogging.SetDefaultLogger(klogging.NewLogrusLogger(ctx).SetConfig(ctx, "debug", "text"))

	// 配置测试环境
	setup := NewFakeTimeTestSetup(t)
	setup.SetupBasicConfig(ctx)
	t.Logf("测试环境已配置")

	fn := func() {
		// Step 1: 创建 ServiceState
		ss := AssembleSsWithShadowState(ctx, "TestAssembleFakeSolver")
		ss.SolverGroup = setup.FakeSnapshotListener
		setup.ServiceState = ss
		t.Logf("ServiceState已创建: %s", ss.Name)

		// Step 2: 创建 worker-1 eph
		workerFullId, _ := setup.CreateAndSetWorkerEph(t, "worker-1", "session-1", "localhost:8080")

		{
			// 等待worker state创建
			waitSucc, elapsedMs := setup.WaitUntilWorkerStateEnum(t, workerFullId, data.WS_Online_healthy, 1000, 10)
			assert.Equal(t, true, waitSucc, "worker state 已创建 elapsedMs=%d", elapsedMs)
		}

		// Step 3: 创建 shardPlan and set into etcd
		// 添加三个分片 (shard-a, shard-b, shard-c)
		setup.FakeTime.VirtualTimeForward(ctx, 10)
		firstShardPlan := []string{"shard-a", "shard-b", "shard-c"}
		setShardPlan(t, setup.FakeEtcd, ctx, firstShardPlan)

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
		{
			shardId := data.ShardId("shard-a")
			replicaFullId := data.NewReplicaFullId(shardId, 0)
			move := costfunc.NewAssignMove(replicaFullId, data.AssignmentId("as1"), workerFullId)
			proposal := costfunc.NewProposal(ctx, "TestAssembleFakeSolver", costfunc.Gain{HardScore: 1}, setup.FakeSnapshotListener.snapshot.SnapshotId)
			proposal.Move = move
			ss.ProposalQueue.Push(proposal)
		}

		// Step 5: 等待accept (接受提案)
		{
			// 等待提案接受
			waitSucc, elapsedMs := WaitUntil(t, func() (bool, string) {
				if ss.AcceptedCount == 1 {
					return true, ""
				}
				return false, "accept数目未达预期"
			}, 1000, 100)
			assert.True(t, waitSucc, "应该能在超时前接受提案, 耗时=%dms", elapsedMs)
		}
		{
			// 等待 move
			waitSucc, elapsedMs := WaitUntil(t, func() (bool, string) {
				if len(ss.AllMoves) == 1 {
					return true, ""
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
					return true, ""
				}
				return false, "assignment 数目未达预期"
			}, 1000, 10)
			assert.True(t, waitSucc, "应该能在超时前 update pilotNode, 耗时=%dms", elapsedMs)
		}

		// Step 6: Fake worker eph
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
					for assignId := range ws.Assignments {
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
		{
			// 等待 routing state
			waitSucc, elapsedMs := setup.WaitUntilRoutingState(t, workerFullId, func(rj *unicornjson.WorkerEntryJson) (bool, string) {
				if rj == nil {
					return false, "routing state 不存在"
				}
				if len(rj.Assignments) == 1 {
					for _, assign := range rj.Assignments {
						if assign.AsginmentId == "as1" {
							return true, ""
						}
					}
				}
				return false, "assignment 数目未达预期"
			}, 1000, 10)
			assert.True(t, waitSucc, "应该能在超时前 update routing state, 耗时=%dms", elapsedMs)
		}

		// stop
		ss.StopAndWaitForExit(ctx)
		setup.PrintAll(ctx)

		// assert.Equal(t, true, false, "") // 强制查看测试输出
	}
	// 使用 FakeTimeProvider 和模拟的 EtcdProvider/EtcdStore 运行测试
	setup.RunWith(fn)
}

/*
------------------ FakeEtcdStore.PrintAll -----------------
<routing>
key = /smg/routing/worker-1:session-1
value =
{
  "worker_id": "worker-1",
  "addr_port": "localhost:8080",
  "capacity": 60,
  "assignments": [
    {
      "shd": "shard-a",
      "asg": "as1"
    }
  ],
  "update_time_ms": 1234500000520,
  "update_reason": "asign1,Assign"
}

key = /smg/worker_state/worker-1:session-1
value =
{
  "worker_id": "worker-1",
  "session_id": "session-1",
  "worker_state": "online_healthy",
  "assignments": {
    "as1": {
      "sid": "shard-a"
    }
  },
  "update_time_ms": 1234500000520,
  "update_reason": "asign1,Assign"
}

key=/smg/shard_state/shard-a, value={"shard_name":"shard-a","resplicas":{"0":{"assignments":["as1"]}}}
key=/smg/shard_state/shard-b, value={"shard_name":"shard-b","resplicas":{"0":{}}}
key=/smg/shard_state/shard-c, value={"shard_name":"shard-c","resplicas":{"0":{}}}

key=/smg/move/FJVZCYK8, value=
{
  "proposal_id": "FJVZCYK8",
  "signature": "shard-a:0/worker-1:session-1",
  "moves": [
    {
      "name": "add_shard",
      "shard": "shard-a",
      "assignment": "as1",
      "to": "worker-1:session-1"
    },
    {
      "name": "add_to_routing",
      "shard": "shard-a",
      "assignment": "as1",
      "to": "worker-1:session-1"
    }
  ],
  "next_move": 0
}

key=/smg/pilot/worker-1:session-1, value=
{
  "worker_id": "worker-1",
  "session_id": "session-1",
  "assignments": [
    {
      "shd": "shard-a",
      "asg": "as1",
      "sts": "ready"
    }
  ],
  "update_time_ms": 1234500000520,
  "update_reason": "asign1,Assign"
}

------------------ FakeEtcdProvider.PrintAll -----------------

key=/smg/config/shard_plan.txt, value=shard-a
shard-b
shard-c

key=/smg/config/service_info.json, value={"service_name":"test-service","stateful_type":"state_in_mem","move_type":"start_before_kill","max_replica_count":10,"min_replica_count":1}

key=/smg/config/service_config.json, value=
{
  "shard_config": {
    "move_policy": "start_before_kill",
    "max_replica_count": 10,
    "min_replica_count": 1
  },
  "worker_config": {
    "MaxAssignmentCountPerWorker": 10,
    "OfflineGracePeriodSec": 60
  },
  "system_limit": {
    "max_shards_count_limit": 1000,
    "max_replica_count_limit": 1000,
    "max_assignment_count_limit": 1000,
    "max_hat_count_count": 10
  },
  "cost_func_cfg": {
    "shard_count_cost_enable": true,
    "shard_count_cost_norm": 100,
    "worker_max_assignments": 100
  },
  "solver_config": {
    "soft_solver_config": {
      "soft_solver_enabled": true,
      "run_per_minute": 30,
      "explore_per_run": 100
    },
    "assign_solver_config": {
      "assign_solver_enabled": true,
      "run_per_minute": 30,
      "explore_per_run": 100
    },
    "unassign_solver_config": {
      "unassign_solver_enabled": false,
      "run_per_minute": 30,
      "explore_per_run": 100
    }
  }
}

key=/smg/eph/worker-1:session-1, value=
{
  "worker_id": "worker-1",
  "session_id": "session-1",
  "addr_port": "localhost:8080",
  "start_time_ms": 1234567890000,
  "capacity": 60,
  "assignments": [
    {
      "shd": "shard-a",
      "asg": "as1",
      "sts": "ready"
    }
  ],
  "update_time_ms": 1234500000510,
  "update_reason": "AddShard"
}
*/
