package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/libs/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/libs/unicorn/unicornjson"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

var (
	workerState1  = `{"worker_id":"worker-1","session_id":"XO4XAC28","worker_state":"online_healthy","assignments":{"7KQGUNAW":{"sid":"shard_40_80","current_state":"ready","target_state":"ready","in_routing":1},"AUW4KJR8":{"sid":"shard_00_40","current_state":"ready","target_state":"ready","in_routing":1}},"update_time_ms":1747894546856,"update_reason":"addToRouting","stateful_type":"state_in_mem"}`
	workerPilot1  = `{"worker_id":"worker-1","session_id":"XO4XAC28","assignments":[{"shd":"shard_00_40","asg":"AUW4KJR8","sts":"ready"},{"shd":"shard_40_80","asg":"7KQGUNAW","sts":"ready"}],"update_time_ms":1747894546855,"update_reason":"updateAssign:7KQGUNAW:ready"}`
	workerRouting = `{"worker_id":"worker-1","addr_port":"localhost:8081","assignments":[{"shd":"shard_00_40","asg":"AUW4KJR8"},{"shd":"shard_40_80","asg":"7KQGUNAW"}],"update_time_ms":1747894546856,"update_reason":"addToRouting"}`
)

func TestAssembleWorkerInitCleanup(t *testing.T) {
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
		// Step 1: prepare shardPlan and set into etcd
		klogging.Info(ctx).Log("Step1", "创建 shardPlan")
		workerFullId := data.WorkerFullIdParseFromString("worker-1:XO4XAC28")
		{
			setup.SetShardPlan(ctx, []string{"shard_00_40", "shard_40_80", "shard_80_c0", "shard_c0_00"})
			setup.ModifyWorkerState(ctx, workerFullId, func(wej *smgjson.WorkerStateJson) *smgjson.WorkerStateJson {
				return smgjson.WorkerStateJsonFromJson(workerState1)
			})
			setup.ModifyPilotNode(ctx, workerFullId, func(pnj *cougarjson.PilotNodeJson) *cougarjson.PilotNodeJson {
				return cougarjson.ParsePilotNodeJson(workerPilot1)
			})
			setup.ModifyRoutingEntry(ctx, workerFullId, func(rj *unicornjson.WorkerEntryJson) *unicornjson.WorkerEntryJson {
				return unicornjson.WorkerEntryJsonFromJson(workerRouting)
			})
			setup.ModifyShardState(ctx, "shard_00_40", func(ssj *smgjson.ShardStateJson) *smgjson.ShardStateJson {
				return smgjson.ShardStateJsonFromJson(`{"shard_name":"shard_00_40","resplicas":{"0":{}},"target_replica_count":1,"last_update_reason":"unmarshal"}`)
			})
			setup.ModifyShardState(ctx, "shard_40_80", func(ssj *smgjson.ShardStateJson) *smgjson.ShardStateJson {
				return smgjson.ShardStateJsonFromJson(`{"shard_name":"shard_40_80","resplicas":{"0":{}},"target_replica_count":1,"last_update_reason":"unmarshal"}`)
			})
			setup.ModifyShardState(ctx, "shard_80_c0", func(ssj *smgjson.ShardStateJson) *smgjson.ShardStateJson {
				return smgjson.ShardStateJsonFromJson(`{"shard_name":"shard_80_c0","resplicas":{"0":{}},"target_replica_count":1,"last_update_reason":"unmarshal"}`)
			})
			setup.ModifyShardState(ctx, "shard_c0_00", func(ssj *smgjson.ShardStateJson) *smgjson.ShardStateJson {
				return smgjson.ShardStateJsonFromJson(`{"shard_name":"shard_c0_00","resplicas":{"0":{}},"target_replica_count":1,"last_update_reason":"unmarshal"}`)
			})
		}

		// Step 2: 创建 ServiceState
		klogging.Info(ctx).Log("Step2", "创建 ServiceState")
		ss := AssembleSsAll(ctx, "TestAssembleAssignSolver")
		setup.ServiceState = ss
		klogging.Info(ctx).Log("ServiceState已创建", ss.Name)

		// Step 3: will not cleanup, since unable to find new home for those shards
		klogging.Info(ctx).Log("Step3", "等待 VirtualTimeForward")
		setup.FakeTime.VirtualTimeForward(ctx, 30*1000)
		{
			var acceptCount int
			setup.safeAccessServiceState(func(ss *ServiceState) {
				acceptCount = ss.AcceptedCount
			})
			assert.Equal(t, 0, acceptCount, "应该没有 proposal 被接受")
		}

		// Step 4: add another worker (worker-2)
		klogging.Info(ctx).Log("Step4", "添加 worker-2")
		workerFullId2, _ := setup.CreateAndSetWorkerEph(t, "worker-2", "session-2", "localhost:8082")

		{
			// 等待 worker-2 的 pilot 节点更新
			var assignments []*cougarjson.PilotAssignmentJson
			waitSucc, elapsedMs := setup.WaitUntilPilotNode(t, workerFullId2, func(pnj *cougarjson.PilotNodeJson) (bool, string) {
				if pnj == nil {
					return false, "没有 pilot 节点"
				}
				if len(pnj.Assignments) < 4 {
					return false, pnj.ToJson()
				}
				assignments = pnj.Assignments
				return true, pnj.ToJson()
			}, 30*1000, 1000)
			assert.Equal(t, true, waitSucc, "应该能在超时前 pilotNode update, 耗时=%dms", elapsedMs)

			setup.UpdateEphNode(workerFullId2, func(wej *cougarjson.WorkerEphJson) *cougarjson.WorkerEphJson {
				for _, assign := range assignments {
					assItem := cougarjson.NewAssignmentJson(assign.ShardId, assign.ReplicaIdx, assign.AsginmentId, cougarjson.CAS_Ready)
					wej.Assignments = append(wej.Assignments, assItem)
				}
				wej.LastUpdateAtMs = kcommon.GetWallTimeMs()
				wej.LastUpdateReason = "simulate eph node update"
				return wej
			})
		}

		// Step 5: since worker eph does not exist, all those worker pilot/routing/worker state should be cleaned up
		klogging.Info(ctx).Log("Step5", "wait for cleaned up")
		{
			waitSucc, elapsedMs := setup.WaitUntilWorkerState(t, workerFullId, func(ws *WorkerState) (bool, string) {
				if ws == nil {
					return true, "worker state cleaned up"
				}
				return false, ws.ToFullString()
			}, 30*1000, 1000)
			assert.Equal(t, true, waitSucc, "应该能在超时前 worker state cleaned up, 耗时=%dms", elapsedMs)
		}
		{
			waitSucc, elapsedMs := setup.WaitUntilPilotNode(t, workerFullId, func(pnj *cougarjson.PilotNodeJson) (bool, string) {
				if pnj == nil {
					return true, "pilot node cleaned up"
				}
				return false, pnj.ToJson()
			}, 30*1000, 1000)
			assert.Equal(t, true, waitSucc, "应该能在超时前 pilot node cleaned up, 耗时=%dms", elapsedMs)
		}
		{
			waitSucc, elapsedMs := setup.WaitUntilRoutingState(t, workerFullId, func(rj *unicornjson.WorkerEntryJson) (bool, string) {
				if rj == nil {
					return true, "routing entry cleaned up"
				}
				return false, rj.ToJson()
			}, 30*1000, 1000)
			assert.Equal(t, true, waitSucc, "应该能在超时前 routing entry cleaned up, 耗时=%dms", elapsedMs)
		}
	}

	// 使用 FakeTimeProvider 和模拟的 EtcdProvider/EtcdStore 运行测试
	setup.RunWith(fn)
}
