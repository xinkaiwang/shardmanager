package costfunc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

func TestCostfunc_basic(t *testing.T) {
	ctx := context.Background()
	cfg := createDefaultCostConfig()

	// step 1: Empty snapshot, cost should be 0
	snap1 := NewSnapshot(ctx, cfg)
	{
		cost1 := snap1.GetCost()
		assert.Equal(t, NewCost(0, 0), cost1, "初始硬成本应为0")
	}

	// step 2: Add a shard (no replica yet), hard cost should be 0
	snap2 := snap1.Clone()
	{
		shard := NewShardSnap("shard_1")
		snap2.AllShards.Set("shard_1", shard)
		cost2 := snap2.GetCost()
		assert.Equal(t, cost2.HardScore, int32(0), "添加shard后硬成本应为0")
	}

	// step 3: Add a replica (not assigned yet), hard cost should be >0
	snap3 := snap2.Clone()
	workerFullId := data.WorkerFullIdParseFromString("worker-1:session-1")
	{
		move1 := NewReplicaAddRemove(data.ShardId("shard_1"), 0, true)
		move1.Apply(snap3)
		move2 := NewWorkerAdded(workerFullId, NewWorkerSnap(workerFullId))
		move2.Apply(snap3)
		cost3 := snap3.GetCost()
		assert.Greater(t, cost3.HardScore, int32(0), "添加副本后硬成本应该大于0")
	}

	// step 4: Assign a replica to a worker, hard cost should be 0
	snap4 := snap3.Clone()
	{
		move1 := NewAssignMove(data.NewReplicaFullId("shard_1", 0), "assign1", workerFullId)
		move1.Apply(snap4, AM_Strict)
		cost4 := snap4.GetCost()
		cost3 := snap3.GetCost()
		assert.Equal(t, true, cost4.HardScore < cost3.HardScore, "hard cost should get lower after assign")
		assert.Equal(t, true, cost4.SoftScore > cost3.SoftScore, "soft cost should get slightly higher after assign")
		assert.Equal(t, true, cost4.IsLowerThan(cost3), "cost should get lower after assign")
	}

	// step 5: unassign a replica from a worker, hard cost should be >0
	snap5 := snap4.Clone()
	{
		move1 := NewUnassignMove(workerFullId, data.NewReplicaFullId("shard_1", 0), "assign1")
		move1.Apply(snap5, AM_Strict)
		cost5 := snap5.GetCost()
		cost4 := snap4.GetCost()
		assert.Equal(t, true, cost5.HardScore > cost4.HardScore, "hard cost should get higher after unassign")
		assert.Equal(t, true, cost5.SoftScore < cost4.SoftScore, "soft cost should get slightly lower after unassign")
	}
}
