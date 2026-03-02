package core

import (
	"log/slog"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/libs/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/libs/unicorn/unicornjson"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

func TestAssembleWorkerInitCleanup2(t *testing.T) {
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
		// Step 1: prepare shardPlan and set into etcd
		slog.InfoContext(ctx, "创建 shardPlan",
			slog.String("event", "Step1"))
		worker2FullId := data.WorkerFullIdParseFromString("worker-2:OK19G93V")
		{
			setup.SetShardPlan(ctx, []string{"shard_00_40", "shard_40_80", "shard_80_c0", "shard_c0_00"})
			setup.ModifyWorkerState(ctx, worker2FullId, func(wej *smgjson.WorkerStateJson) *smgjson.WorkerStateJson {
				return smgjson.WorkerStateJsonFromJson(`{"worker_id":"worker-2","session_id":"OK19G93V","worker_state":"offline_draining_candidate","assignments":{"KY87YWQW":{"sid":"shard_00_40","current_state":"unkonwn","target_state":"unkonwn"},"YYV972IU":{"sid":"shard_40_80","current_state":"unkonwn","target_state":"unkonwn"}},"update_time_ms":1747894514035,"update_reason":"WS_Offline_draining_candidate","hat":1,"stateful_type":"state_in_mem"}`)
			})
			setup.ModifyPilotNode(ctx, worker2FullId, func(pnj *cougarjson.PilotNodeJson) *cougarjson.PilotNodeJson {
				return cougarjson.ParsePilotNodeJson(`{"worker_id":"worker-2","session_id":"OK19G93V","assignments":[],"update_time_ms":1747894514035,"update_reason":"WS_Offline_draining_candidate"}`)
			})
			setup.ModifyRoutingEntry(ctx, worker2FullId, func(rj *unicornjson.WorkerEntryJson) *unicornjson.WorkerEntryJson {
				return unicornjson.WorkerEntryJsonFromJson(`{"worker_id":"worker-2","addr_port":"localhost:8082","assignments":[{"shd":"shard_80_c0","asg":"4Z1HJW58"},{"shd":"shard_c0_00","asg":"MOSQOYP2"}],"update_time_ms":1747894548046,"update_reason":"unassign:EFXM8D1G:dropped"}`)
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
		slog.InfoContext(ctx, "创建 ServiceState",
			slog.String("event", "Step2"))
		ss := AssembleSsAll(ctx, "TestAssembleAssignSolver")
		setup.ServiceState = ss
		slog.InfoContext(ctx, ss.Name,
			slog.String("event", "ServiceState已创建"))

		// Step 3: will not cleanup, since unable to find new home for those shards
		setup.FakeTime.VirtualTimeForward(ctx, 30*1000)
		{
			var acceptCount int
			setup.safeAccessServiceState(func(ss *ServiceState) {
				acceptCount = ss.AcceptedCount
			})
			assert.Equal(t, 0, acceptCount, "应该没有 proposal 被接受")
		}

		// Step 4: add another worker (worker-3)
		slog.InfoContext(ctx, "添加 worker-3",
			slog.String("event", "Step4"))
		workerFullId3, _ := setup.CreateAndSetWorkerEph(t, "worker-3", "session-3", "localhost:8083")

		{
			// 等待 worker-2 的 pilot 节点更新
			var assignments []*cougarjson.PilotAssignmentJson
			waitSucc, elapsedMs := setup.WaitUntilPilotNode(t, workerFullId3, func(pnj *cougarjson.PilotNodeJson) (bool, string) {
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

			setup.UpdateEphNode(workerFullId3, func(wej *cougarjson.WorkerEphJson) *cougarjson.WorkerEphJson {
				for _, assign := range assignments {
					assItem := cougarjson.NewAssignmentJson(assign.ShardId, assign.ReplicaIdx, assign.AssignmentId, cougarjson.CAS_Ready)
					wej.Assignments = append(wej.Assignments, assItem)
				}
				wej.LastUpdateAtMs = kcommon.GetWallTimeMs()
				wej.LastUpdateReason = "simulate eph node update"
				return wej
			})
		}

		// Step 5: since worker eph does not exist, all those worker pilot/routing/worker state should be cleaned up
		slog.InfoContext(ctx, "wait for cleaned up",
			slog.String("event", "Step5"))
		{
			waitSucc, elapsedMs := setup.WaitUntilWorkerState(t, worker2FullId, func(ws *WorkerState) (bool, string) {
				if ws == nil {
					return true, "worker state cleaned up"
				}
				return false, ws.ToFullString()
			}, 30*1000, 1000)
			assert.Equal(t, true, waitSucc, "应该能在超时前 worker state cleaned up, 耗时=%dms", elapsedMs)
		}
		{
			waitSucc, elapsedMs := setup.WaitUntilPilotNode(t, worker2FullId, func(pnj *cougarjson.PilotNodeJson) (bool, string) {
				if pnj == nil {
					return true, "pilot node cleaned up"
				}
				return false, pnj.ToJson()
			}, 30*1000, 1000)
			assert.Equal(t, true, waitSucc, "应该能在超时前 pilot node cleaned up, 耗时=%dms", elapsedMs)
		}
		{
			waitSucc, elapsedMs := setup.WaitUntilRoutingState(t, worker2FullId, func(rj *unicornjson.WorkerEntryJson) (bool, string) {
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
