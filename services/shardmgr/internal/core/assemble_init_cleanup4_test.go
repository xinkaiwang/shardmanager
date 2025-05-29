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

func _TestAssembleWorkerInitCleanup4(t *testing.T) {
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
				return smgjson.WorkerStateJsonFromJson(`{"worker_id":"smgapp-6d9cfc6cdd-b9whq","session_id":"6X30CVPL","worker_info":{"address_port":"10.108.1.151:8080","start_time_ms":1748392249064,"capacity":1000,"memory_size_mb":0},"worker_state":"online_healthy","assignments":{"1K5CSS5K":{"sid":"shard_80_c0","rid":4,"current_state":"ready","target_state":"ready","in_pilot":1,"in_routing":1},"7DTBC24R":{"sid":"shard_80_c0","rid":1,"current_state":"dropped","target_state":"ready","in_routing":1},"DRGNL5RL":{"sid":"shard_80_c0","rid":2,"current_state":"dropped","target_state":"ready","in_routing":1},"DV9WK3DJ":{"sid":"shard_80_c0","current_state":"dropped","target_state":"ready","in_routing":1},"F7Z2D89Z":{"sid":"shard_40_80","rid":1,"current_state":"dropped","target_state":"ready","in_routing":1},"G3YR6XMX":{"sid":"shard_40_80","rid":3,"current_state":"dropped","target_state":"ready","in_routing":1},"O943MMQP":{"sid":"shard_00_40","rid":3,"current_state":"dropped","target_state":"ready","in_routing":1},"ROCQ8S56":{"sid":"shard_c0_00","current_state":"dropped","target_state":"ready","in_routing":1},"S1VB4C72":{"sid":"shard_40_80","rid":4,"current_state":"ready","target_state":"ready","in_pilot":1,"in_routing":1},"VXU99ZAZ":{"sid":"shard_40_80","rid":2,"current_state":"dropped","target_state":"ready","in_routing":1}},"update_time_ms":1234500000000,"update_reason":"WorkerInfo","stateful_type":"state_in_mem"}`)
			})
			setup.ModifyPilotNode(ctx, worker1FullId, func(pnj *cougarjson.PilotNodeJson) *cougarjson.PilotNodeJson {
				return cougarjson.ParsePilotNodeJson(`{"worker_id":"smgapp-6d9cfc6cdd-b9whq","session_id":"6X30CVPL","assignments":[{"shd":"shard_80_c0","idx":4,"asg":"1K5CSS5K","sts":"ready"},{"shd":"shard_40_80","idx":4,"asg":"S1VB4C72","sts":"ready"}],"update_time_ms":1748481151040,"update_reason":"updateAssign:1K5CSS5K:ready"}`)
			})
			setup.ModifyRoutingEntry(ctx, worker1FullId, func(rj *unicornjson.WorkerEntryJson) *unicornjson.WorkerEntryJson {
				return unicornjson.WorkerEntryJsonFromJson(`{"worker_id":"smgapp-6d9cfc6cdd-b9whq","session_id":"6X30CVPL","addr_port":"10.108.1.151:8080","assignments":[{"shd":"shard_80_c0","idx":4,"asg":"1K5CSS5K"},{"shd":"shard_80_c0","idx":1,"asg":"7DTBC24R"},{"shd":"shard_40_80","idx":3,"asg":"G3YR6XMX"},{"shd":"shard_40_80","idx":1,"asg":"F7Z2D89Z"},{"shd":"shard_00_40","idx":3,"asg":"O943MMQP"},{"shd":"shard_c0_00","asg":"ROCQ8S56"},{"shd":"shard_40_80","idx":2,"asg":"VXU99ZAZ"},{"shd":"shard_40_80","idx":4,"asg":"S1VB4C72"},{"shd":"shard_80_c0","idx":2,"asg":"DRGNL5RL"},{"shd":"shard_80_c0","asg":"DV9WK3DJ"}],"update_time_ms":1748481151040,"update_reason":"addToRouting"}`)
			})
			setup.ModifyWorkerState(ctx, worker2FullId, func(wej *smgjson.WorkerStateJson) *smgjson.WorkerStateJson {
				return smgjson.WorkerStateJsonFromJson(`{"worker_id":"smgapp-6d9cfc6cdd-t7mqx","session_id":"KV57TEHC","worker_state":"online_healthy","assignments":{"0N3BMAOX":{"sid":"shard_c0_00","rid":3,"current_state":"dropped","target_state":"ready","in_routing":1},"7VLPERL7":{"sid":"shard_00_40","rid":1,"current_state":"dropped","target_state":"ready","in_routing":1},"A2ADCRWC":{"sid":"shard_c0_00","rid":2,"current_state":"dropped","target_state":"ready","in_routing":1},"CO8E9IXV":{"sid":"shard_c0_00","rid":1,"current_state":"dropped","target_state":"ready","in_routing":1},"D68K3VQ1":{"sid":"shard_40_80","current_state":"dropped","target_state":"ready","in_routing":1},"E41NKQUT":{"sid":"shard_00_40","rid":4,"current_state":"ready","target_state":"ready","in_pilot":1,"in_routing":1},"M5BYI6QP":{"sid":"shard_00_40","rid":2,"current_state":"dropped","target_state":"ready","in_routing":1},"NFQFZQZL":{"sid":"shard_00_40","current_state":"dropped","target_state":"ready","in_routing":1},"UVZMJNN4":{"sid":"shard_c0_00","rid":4,"current_state":"ready","target_state":"ready","in_pilot":1,"in_routing":1},"Z70GLRV1":{"sid":"shard_80_c0","rid":3,"current_state":"dropped","target_state":"ready","in_routing":1}},"update_time_ms":1748481145042,"update_reason":"addToRouting","stateful_type":"state_in_mem"}`)
			})
			setup.ModifyPilotNode(ctx, worker2FullId, func(pnj *cougarjson.PilotNodeJson) *cougarjson.PilotNodeJson {
				return cougarjson.ParsePilotNodeJson(`{"worker_id":"smgapp-6d9cfc6cdd-t7mqx","session_id":"KV57TEHC","assignments":[{"shd":"shard_00_40","idx":4,"asg":"E41NKQUT","sts":"ready"},{"shd":"shard_c0_00","idx":4,"asg":"UVZMJNN4","sts":"ready"}],"update_time_ms":1748481145041,"update_reason":"updateAssign:UVZMJNN4:ready"}`)
			})
			setup.ModifyRoutingEntry(ctx, worker2FullId, func(rj *unicornjson.WorkerEntryJson) *unicornjson.WorkerEntryJson {
				return unicornjson.WorkerEntryJsonFromJson(`{"worker_id":"smgapp-6d9cfc6cdd-t7mqx","session_id":"KV57TEHC","addr_port":"10.108.1.239:8080","assignments":[{"shd":"shard_80_c0","idx":3,"asg":"Z70GLRV1"},{"shd":"shard_c0_00","idx":3,"asg":"0N3BMAOX"},{"shd":"shard_00_40","idx":1,"asg":"7VLPERL7"},{"shd":"shard_c0_00","idx":1,"asg":"CO8E9IXV"},{"shd":"shard_40_80","asg":"D68K3VQ1"},{"shd":"shard_00_40","idx":4,"asg":"E41NKQUT"},{"shd":"shard_c0_00","idx":4,"asg":"UVZMJNN4"},{"shd":"shard_c0_00","idx":2,"asg":"A2ADCRWC"},{"shd":"shard_00_40","idx":2,"asg":"M5BYI6QP"},{"shd":"shard_00_40","asg":"NFQFZQZL"}],"update_time_ms":1748481145042,"update_reason":"addToRouting"}`)
			})
			setup.ModifyShardState(ctx, "shard_00_40", func(ssj *smgjson.ShardStateJson) *smgjson.ShardStateJson {
				return smgjson.ShardStateJsonFromJson(`{"shard_name":"shard_00_40","resplicas":{"0":{},"1":{"idx":1},"2":{"idx":2},"3":{"idx":3},"4":{"idx":4}},"target_replica_count":1,"last_update_reason":"unmarshal"}`)
			})
			setup.ModifyShardState(ctx, "shard_40_80", func(ssj *smgjson.ShardStateJson) *smgjson.ShardStateJson {
				return smgjson.ShardStateJsonFromJson(`{"shard_name":"shard_40_80","resplicas":{"0":{},"1":{"idx":1},"2":{"idx":2},"3":{"idx":3},"4":{"idx":4}},"target_replica_count":1,"last_update_reason":"unmarshal"}`)
			})
			setup.ModifyShardState(ctx, "shard_80_c0", func(ssj *smgjson.ShardStateJson) *smgjson.ShardStateJson {
				return smgjson.ShardStateJsonFromJson(`{"shard_name":"shard_80_c0","resplicas":{"0":{},"1":{"idx":1},"2":{"idx":2},"3":{"idx":3},"4":{"idx":4}},"target_replica_count":1,"last_update_reason":"unmarshal"}`)
			})
			setup.ModifyShardState(ctx, "shard_c0_00", func(ssj *smgjson.ShardStateJson) *smgjson.ShardStateJson {
				return smgjson.ShardStateJsonFromJson(`{"shard_name":"shard_c0_00","resplicas":{"0":{"lame_duck":1},"1":{"idx":1},"2":{"idx":2},"3":{"idx":3},"4":{"idx":4}},"target_replica_count":1,"last_update_reason":"unmarshal"}`)
			})
			setup.UpdateEphNode(worker1FullId, func(wej *cougarjson.WorkerEphJson) *cougarjson.WorkerEphJson {
				node := cougarjson.WorkerEphJsonFromJson(`{"worker_id":"smgapp-6d9cfc6cdd-b9whq","session_id":"6X30CVPL","addr_port":"10.108.1.151:8080","start_time_ms":1748392249064,"capacity":1000,"assignments":[{"shd":"shard_80_c0","idx":4,"asg":"1K5CSS5K","sts":"ready","qpm":{"qpm":0}},{"shd":"shard_40_80","idx":4,"asg":"S1VB4C72","sts":"ready","qpm":{"qpm":0}}],"update_time_ms":1748483072793,"update_reason":"stats_report","stateful_type":"state_in_mem"}`)
				return node
			})
			setup.UpdateEphNode(worker2FullId, func(wej *cougarjson.WorkerEphJson) *cougarjson.WorkerEphJson {
				node := cougarjson.WorkerEphJsonFromJson(`{"worker_id":"smgapp-6d9cfc6cdd-t7mqx","session_id":"KV57TEHC","addr_port":"10.108.1.239:8080","start_time_ms":1748392205736,"capacity":1000,"assignments":[{"shd":"shard_00_40","idx":4,"asg":"E41NKQUT","sts":"ready","qpm":{"qpm":0}},{"shd":"shard_c0_00","idx":4,"asg":"UVZMJNN4","sts":"ready","qpm":{"qpm":0}}],"update_time_ms":1748483059363,"update_reason":"stats_report","stateful_type":"state_in_mem"}`)
				return node
			})
		}

		// Step 2: 创建 ServiceState
		klogging.Info(ctx).Log("Step2", "创建 ServiceState")
		ss := AssembleSsAll(ctx, "TestAssembleAssignSolver")
		setup.ServiceState = ss
		klogging.Info(ctx).Log("ServiceState已创建", ss.Name)

		// Step 3: will not cleanup, since unable to find new home for those shards
		setup.FakeTime.VirtualTimeForward(ctx, 60*1000)
		{
			var acceptCount int
			setup.safeAccessServiceState(func(ss *ServiceState) {
				acceptCount = ss.AcceptedCount
			})
			assert.Equal(t, true, acceptCount > 8, "应该有 proposal 被接受")
		}
	}

	// 使用 FakeTimeProvider 和模拟的 EtcdProvider/EtcdStore 运行测试
	setup.RunWith(fn)
}
