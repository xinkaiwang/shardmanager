package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/libs/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/libs/unicorn/unicornjson"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

func TestAssembleWorkerInitCleanup3(t *testing.T) {
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
		// Step 1: prepare shardPlan and set into etcd
		klogging.Info(ctx).Log("Step1", "创建 shardPlan")
		worker1FullId := data.WorkerFullIdParseFromString("smgapp-6d9cfc6cdd-b9whq:6X30CVPL")
		worker2FullId := data.WorkerFullIdParseFromString("smgapp-6d9cfc6cdd-t7mqx:KV57TEHC")
		{
			setup.SetShardPlan(ctx, []string{"shard_00_40", "shard_40_80", "shard_80_c0", "shard_c0_00"})
			setup.ModifyWorkerState(ctx, worker1FullId, func(wej *smgjson.WorkerStateJson) *smgjson.WorkerStateJson {
				return smgjson.WorkerStateJsonFromJson(`{"worker_id":"smgapp-6d9cfc6cdd-b9whq","session_id":"6X30CVPL","worker_state":"online_healthy","assignments":{"DV9WK3DJ":{"sid":"shard_80_c0","current_state":"dropped","target_state":"ready","in_routing":1},"ROCQ8S56":{"sid":"shard_c0_00","current_state":"dropped","target_state":"ready","in_routing":1}},"update_time_ms":1748394211168,"update_reason":"WorkerInfo,unassign:DV9WK3DJ:dropped,unassign:ROCQ8S56:dropped","stateful_type":"state_in_mem"}`)
			})
			setup.ModifyPilotNode(ctx, worker1FullId, func(pnj *cougarjson.PilotNodeJson) *cougarjson.PilotNodeJson {
				return cougarjson.ParsePilotNodeJson(`{"worker_id":"smgapp-6d9cfc6cdd-b9whq","session_id":"6X30CVPL","assignments":[],"update_time_ms":1748394211168,"update_reason":"WorkerInfo,unassign:DV9WK3DJ:dropped,unassign:ROCQ8S56:dropped"}`)
			})
			setup.ModifyRoutingEntry(ctx, worker1FullId, func(rj *unicornjson.WorkerEntryJson) *unicornjson.WorkerEntryJson {
				return unicornjson.WorkerEntryJsonFromJson(`{"worker_id":"smgapp-6d9cfc6cdd-b9whq","session_id":"6X30CVPL","addr_port":"10.108.1.151:8080","assignments":[{"shd":"shard_80_c0","asg":"DV9WK3DJ"},{"shd":"shard_c0_00","asg":"ROCQ8S56"}],"update_time_ms":1748394211168,"update_reason":"WorkerInfo,unassign:DV9WK3DJ:dropped,unassign:ROCQ8S56:dropped"}`)
			})
			setup.ModifyWorkerState(ctx, worker2FullId, func(wej *smgjson.WorkerStateJson) *smgjson.WorkerStateJson {
				return smgjson.WorkerStateJsonFromJson(`{"worker_id":"smgapp-6d9cfc6cdd-t7mqx","session_id":"KV57TEHC","worker_state":"online_healthy","assignments":{"D67YJXAD":{"sid":"shard_c0_00","current_state":"dropped","target_state":"dropped"},"D68K3VQ1":{"sid":"shard_40_80","current_state":"dropped","target_state":"ready","in_routing":1},"NFQFZQZL":{"sid":"shard_00_40","current_state":"dropped","target_state":"ready","in_routing":1}},"update_time_ms":1748394211169,"update_reason":"WorkerInfo,unassign:D68K3VQ1:dropped,unassign:NFQFZQZL:dropped","stateful_type":"state_in_mem"}`)
			})
			setup.ModifyPilotNode(ctx, worker2FullId, func(pnj *cougarjson.PilotNodeJson) *cougarjson.PilotNodeJson {
				return cougarjson.ParsePilotNodeJson(`{"worker_id":"smgapp-6d9cfc6cdd-t7mqx","session_id":"KV57TEHC","assignments":[],"update_time_ms":1748394211169,"update_reason":"WorkerInfo,unassign:D68K3VQ1:dropped,unassign:NFQFZQZL:dropped"}`)
			})
			setup.ModifyRoutingEntry(ctx, worker2FullId, func(rj *unicornjson.WorkerEntryJson) *unicornjson.WorkerEntryJson {
				return unicornjson.WorkerEntryJsonFromJson(`{"worker_id":"smgapp-6d9cfc6cdd-t7mqx","session_id":"KV57TEHC","addr_port":"10.108.1.239:8080","assignments":[{"shd":"shard_40_80","asg":"D68K3VQ1"},{"shd":"shard_00_40","asg":"NFQFZQZL"}],"update_time_ms":1748394211169,"update_reason":"WorkerInfo,unassign:D68K3VQ1:dropped,unassign:NFQFZQZL:dropped"}`)
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
			setup.UpdateEphNode(worker1FullId, func(wej *cougarjson.WorkerEphJson) *cougarjson.WorkerEphJson {
				node := cougarjson.WorkerEphJsonFromJson(`{"worker_id":"smgapp-6d9cfc6cdd-b9whq","session_id":"6X30CVPL","addr_port":"10.108.1.151:8080","start_time_ms":1748392249064,"capacity":1000,"update_time_ms":1748395489607,"update_reason":"stats_report","stateful_type":"state_in_mem"}`)
				return node
			})
			setup.UpdateEphNode(worker2FullId, func(wej *cougarjson.WorkerEphJson) *cougarjson.WorkerEphJson {
				node := cougarjson.WorkerEphJsonFromJson(`{"worker_id":"smgapp-6d9cfc6cdd-t7mqx","session_id":"KV57TEHC","addr_port":"10.108.1.239:8080","start_time_ms":1748392205736,"capacity":1000,"update_time_ms":1748395566273,"update_reason":"stats_report","stateful_type":"state_in_mem"}`)
				return node
			})
		}

		// Step 2: 创建 ServiceState
		klogging.Info(ctx).Log("Step2", "创建 ServiceState")
		ss := AssembleSsAll(ctx, "TestAssembleAssignSolver")
		setup.ServiceState = ss
		klogging.Info(ctx).Log("ServiceState已创建", ss.Name)

		// Step 3: will not cleanup, since unable to find new home for those shards
		setup.FakeTime.VirtualTimeForward(ctx, 30*1000)
		{
			var acceptCount int
			setup.safeAccessServiceState(func(ss *ServiceState) {
				acceptCount = ss.AcceptedCount
			})
			assert.Equal(t, 4, acceptCount, "应该有 4 proposal 被接受")
		}
	}

	// 使用 FakeTimeProvider 和模拟的 EtcdProvider/EtcdStore 运行测试
	setup.RunWith(fn)
}
