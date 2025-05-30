package costfunc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

func TestCostfunc_2replica(t *testing.T) {
	ctx := context.Background()
	cfg := createDefaultCostConfig()
	klogging.SetDefaultLogger(klogging.NewLogrusLogger(ctx).SetConfig(ctx, "debug", "simple"))

	// step 1: Empty snapshot, cost should be 0
	klogging.Info(ctx).Log("Step1", "创建空的快照")
	snap1 := NewSnapshot(ctx, cfg)
	{
		cost1 := snap1.GetCost()
		assert.Equal(t, NewCost(0, 0), cost1, "初始硬成本应为0")
	}

	// step 2: Add 1 shards (with 2 replicas), hard cost should be > 0
	klogging.Info(ctx).Log("Step2", "添加一个shard")
	snap2 := snap1.Clone()
	shardId1 := data.ShardId("shard_1")
	{
		shard1 := NewShardSnap(shardId1, 2)
		NewPasMoveShardStateAddRemove(shardId1, shard1, "test").Apply(snap2)

		cost2 := snap2.GetCost()
		assert.Greater(t, cost2.HardScore, int32(0), "添加shard后硬成本应大于0")
	}

	// step 3: Add 2 workers, hard cost should be > 0
	klogging.Info(ctx).Log("Step3", "添加两个worker")
	snap3 := snap2.Clone()
	workerFullId1 := data.WorkerFullIdParseFromString("worker-1:session-1")
	workerFullId2 := data.WorkerFullIdParseFromString("worker-2:session-2")
	{
		NewPasMoveWorkerSnapAddRemove(workerFullId1, NewWorkerSnap(workerFullId1), "").Apply(snap3)
		NewPasMoveWorkerSnapAddRemove(workerFullId2, NewWorkerSnap(workerFullId2), "").Apply(snap3)
		cost3 := snap3.GetCost()
		assert.Greater(t, cost3.HardScore, int32(0), "添加worker后硬成本应大于0")
	}

	// step 4: Assign 1 replica to worker1, hard cost should be > 0
	klogging.Info(ctx).Log("Step4", "分配一个副本到worker1")
	snap4 := snap3.Clone()
	{
		move1 := NewAssignMove(data.NewReplicaFullId(shardId1, 0), "assign1", workerFullId1)
		move1.Apply(snap4, AM_Strict)
		cost4 := snap4.GetCost()
		cost3 := snap3.GetCost()
		assert.Equal(t, true, cost4.HardScore < cost3.HardScore, "hard cost should get lower after assign")
		assert.Equal(t, true, cost4.SoftScore > cost3.SoftScore, "soft cost should get slightly higher after assign")
	}

	// step 5: Assign 1 replica to worker2, hard cost should be = 0
	klogging.Info(ctx).Log("Step5", "分配另一个副本到worker2")
	snap5 := snap4.Clone()
	{
		move1 := NewAssignMove(data.NewReplicaFullId(shardId1, 1), "assign2", workerFullId2)
		move1.Apply(snap5, AM_Strict)
		cost5 := snap5.GetCost()
		cost4 := snap4.GetCost()
		assert.Equal(t, true, cost5.HardScore < cost4.HardScore, "hard cost should get lower after assign")
		assert.Equal(t, true, cost5.SoftScore > cost4.SoftScore, "soft cost should get slightly higher after assign")
	}

	// step 6: Assign replica 1 to worker1, should be illegal
	klogging.Info(ctx).Log("Step6", "尝试将副本1分配给worker1，应该是非法操作")
	snap6 := snap4.Clone()
	{
		move1 := NewAssignMove(data.NewReplicaFullId(shardId1, 1), "assign2", workerFullId1)
		ke := kcommon.TryCatchRun(ctx, func() {
			move1.Apply(snap6, AM_Strict)
		})
		assert.NotNil(t, ke, "assign replica 1 to worker1 should be illegal")
	}
}
