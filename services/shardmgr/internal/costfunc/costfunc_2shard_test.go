package costfunc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

func TestCostfunc_2shard(t *testing.T) {
	ctx := context.Background()
	cfg := createDefaultCostConfig()

	// step 1: Empty snapshot, cost should be 0
	snap1 := NewSnapshot(ctx, cfg)
	{
		cost1 := snap1.GetCost(ctx)
		assert.Equal(t, NewCost(0, 0), cost1, "初始硬成本应为0")
	}

	// step 2: Add 2 shards (with 1 replica each), hard cost should be > 0
	snap2 := snap1.Clone()
	shardId1 := data.ShardId("shard_1")
	shardId2 := data.ShardId("shard_2")
	{
		shard1 := NewShardSnap(shardId1, 1)
		shard2 := NewShardSnap(shardId2, 1)
		NewPasMoveShardStateAddRemove(shardId1, shard1, "").Apply(snap2)
		NewPasMoveShardStateAddRemove(shardId2, shard2, "").Apply(snap2)

		cost2 := snap2.GetCost(ctx)
		assert.Greater(t, cost2.HardScore, int32(0), "添加shard后硬成本应大于0")
	}

	// step 3: Add 2 workers, hard cost should be > 0
	snap3 := snap2.Clone()
	workerFullId1 := data.WorkerFullIdParseFromString("worker-1:session-1")
	workerFullId2 := data.WorkerFullIdParseFromString("worker-2:session-2")
	{
		NewPasMoveWorkerSnapAddRemove(workerFullId1, NewWorkerSnap(workerFullId1), "").Apply(snap3)
		NewPasMoveWorkerSnapAddRemove(workerFullId2, NewWorkerSnap(workerFullId2), "").Apply(snap3)
		cost3 := snap3.GetCost(ctx)
		assert.Greater(t, cost3.HardScore, int32(0), "添加worker后硬成本应大于0")
	}

	// step 4: Assign
	snap4 := snap3.Clone()
	{
		move1 := NewAssignMove(data.NewReplicaFullId(shardId1, 0), "assign1", workerFullId1)
		move1.Apply(snap4, AM_Strict)
		move2 := NewAssignMove(data.NewReplicaFullId(shardId2, 0), "assign2", workerFullId2)
		move2.Apply(snap4, AM_Strict)
		cost4 := snap4.GetCost(ctx)
		cost3 := snap3.GetCost(ctx)
		assert.Equal(t, true, cost4.HardScore < cost3.HardScore, "hard cost should get lower after assign")
		assert.Equal(t, true, cost4.SoftScore > cost3.SoftScore, "soft cost should get slightly higher after assign")
	}

	// step 5: assign (sub-optimal) 2 replicas assign to the same worker
	snap5 := snap3.Clone()
	{
		move1 := NewAssignMove(data.NewReplicaFullId(shardId1, 0), "assign1", workerFullId1)
		move1.Apply(snap5, AM_Strict)
		move2 := NewAssignMove(data.NewReplicaFullId(shardId2, 0), "assign2", workerFullId1)
		move2.Apply(snap5, AM_Strict)
		cost4 := snap4.GetCost(ctx)
		cost5 := snap5.GetCost(ctx)
		assert.Equal(t, true, cost5.HardScore == cost4.HardScore, "hard cost should get same")
		assert.Equal(t, true, cost5.SoftScore > cost4.SoftScore, "soft cost should get higher")
	}
}
