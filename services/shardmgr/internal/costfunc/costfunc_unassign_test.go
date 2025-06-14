package costfunc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

func TestCostfunc_unassign(t *testing.T) {
	ctx := context.Background()
	cfg := createDefaultCostConfig()

	// step 1: Empty snapshot, cost should be 0
	snap1 := NewSnapshot(ctx, cfg)
	{
		cost1 := snap1.GetCost(ctx)
		assert.Equal(t, NewCost(0, 0), cost1, "初始硬成本应为0")
	}

	// step 2: Add 1 shards (with 1 replicas), hard cost should be > 0
	snap2 := snap1.Clone()
	shardId1 := data.ShardId("shard_1")
	{
		shard1 := NewShardSnap(shardId1, 1)
		// shard1.Replicas[0] = NewReplicaSnap(shardId1, 0)
		NewPasMoveShardStateAddRemove(shardId1, shard1, "shard_1").Apply(snap2)

		cost2 := snap2.GetCost(ctx)
		assert.Greater(t, cost2.HardScore, int32(0), "添加shard后硬成本应大于0")
	}

	// step 3: Add 1 workers, hard cost should be > 0
	snap3 := snap2.Clone()
	workerFullId1 := data.WorkerFullIdParseFromString("worker-1:session-1")
	{
		NewPasMoveWorkerSnapAddRemove(workerFullId1, NewWorkerSnap(workerFullId1), "").Apply(snap3)
		cost3 := snap3.GetCost(ctx)
		assert.Greater(t, cost3.HardScore, int32(0), "添加worker后硬成本应大于0")
	}

	// step 4: Assign 1 replica to worker1, hard cost should be > 0
	snap4 := snap3.Clone()
	{
		move1 := NewAssignMove(data.NewReplicaFullId(shardId1, 0), "assign1", workerFullId1)
		move1.Apply(snap4, AM_Strict)
		cost4 := snap4.GetCost(ctx)
		cost3 := snap3.GetCost(ctx)
		assert.Equal(t, true, cost4.HardScore < cost3.HardScore, "hard cost should get lower after assign")
		assert.Equal(t, true, cost4.SoftScore > cost3.SoftScore, "soft cost should get slightly higher after assign")
	}

	// step 5: replica becomes lame duck
	snap5 := snap4.Clone()
	{
		move1 := NewPasMoveReplicaSnapUpdate(shardId1, 0, func(rs *ReplicaSnap) {
			rs.LameDuck = true
		}, "LameDuck")
		move1.Apply(snap5)
		cost5 := snap5.GetCost(ctx)
		cost4 := snap4.GetCost(ctx)
		assert.Equal(t, true, cost5.HardScore > cost4.HardScore, "hard cost should get higher after lame duck (due to H5)")
		assert.Equal(t, true, cost5.SoftScore == cost4.SoftScore, "soft cost should get same after lame duck")
	}

	// step 6: unassign a LameDuck replica from a worker, hard cost should be 0
	snap6 := snap5.Clone()
	{
		move1 := NewUnassignMove(workerFullId1, data.NewReplicaFullId(shardId1, 0), "assign1")
		move1.Apply(snap6, AM_Strict)
		cost6 := snap6.GetCost(ctx)
		cost5 := snap5.GetCost(ctx)
		assert.Equal(t, true, cost6.HardScore < cost5.HardScore, "hard cost should get lower after unassign")
		assert.Equal(t, true, cost6.SoftScore < cost5.SoftScore, "soft cost should get lower after unassign")
		assert.Equal(t, true, cost6.IsLowerThan(cost5), "cost should get lower after unassign")
	}
}
