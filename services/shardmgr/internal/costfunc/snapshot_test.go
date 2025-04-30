package costfunc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

func TestShardSnap(t *testing.T) {
	shardId := data.ShardId("shard1")
	shard := NewShardSnap(shardId, 0)

	// 测试 Clone 方法
	cloned := shard.Clone()
	assert.Equal(t, shard.ShardId, cloned.ShardId)
	assert.NotSame(t, shard.Replicas, cloned.Replicas)
}

func TestReplicaSnap(t *testing.T) {
	shardId := data.ShardId("shard1")
	replicaIdx := data.ReplicaIdx(0)
	replica := NewReplicaSnap(shardId, replicaIdx)

	// 测试 GetReplicaFullId 方法
	fullId := replica.GetReplicaFullId()
	assert.Equal(t, data.ShardId("shard1"), fullId.ShardId)
	assert.Equal(t, data.ReplicaIdx(0), fullId.ReplicaIdx)

	// 测试 Clone 方法
	cloned := replica.Clone()
	assert.Equal(t, replica.ShardId, cloned.ShardId)
	assert.Equal(t, replica.ReplicaIdx, cloned.ReplicaIdx)
	assert.NotSame(t, replica.Assignments, cloned.Assignments)
}

func TestAssignmentSnap(t *testing.T) {
	shardId := data.ShardId("shard1")
	replicaIdx := data.ReplicaIdx(0)
	assignmentId := data.AssignmentId("assign1")

	assignment := NewAssignmentSnap(shardId, replicaIdx, assignmentId, data.NewWorkerFullId("worker1", "session1", data.ST_MEMORY))

	// 测试 GetReplicaFullId 方法
	fullId := assignment.GetReplicaFullId()
	assert.Equal(t, shardId, fullId.ShardId)
	assert.Equal(t, replicaIdx, fullId.ReplicaIdx)
}

func TestWorkerSnap(t *testing.T) {
	workerFullId := data.NewWorkerFullId("worker1", "session1", data.ST_MEMORY)

	worker := NewWorkerSnap(workerFullId)

	// 测试 Clone 方法
	cloned := worker.Clone()
	assert.Equal(t, worker.WorkerFullId, cloned.WorkerFullId)
	assert.NotSame(t, worker.Assignments, cloned.Assignments)

	// 测试 CanAcceptAssignment 方法
	shardId := data.ShardId("shard1")
	assert.True(t, worker.CanAcceptAssignment(shardId))

	// 添加分配后测试
	worker.Assignments[shardId] = data.AssignmentId("assign1")
	assert.False(t, worker.CanAcceptAssignment(shardId))
}

func TestSnapshot(t *testing.T) {
	// 测试 Clone 功能
	t.Run("Clone", func(t *testing.T) {
		cfg := config.CostfuncConfig{}
		ctx := context.Background()
		snapshot := NewSnapshot(ctx, cfg)
		snapshot.SnapshotId = SnapshotId("snap1")

		// 添加一个 worker
		workerFullId := data.NewWorkerFullId("worker1", "session1", data.ST_MEMORY)
		worker := NewWorkerSnap(workerFullId)
		snapshot.AllWorkers.Set(workerFullId, worker)

		// 添加一个 shard 和 replica
		shardId := data.ShardId("shard1")
		// replicaIdx := data.ReplicaIdx(0)
		shard := NewShardSnap(shardId, 1)
		// shard.Replicas[replicaIdx] = NewReplicaSnap(shardId, replicaIdx)
		snapshot.AllShards.Set(shardId, shard)

		// 在克隆前先冻结快照
		snapshot.Freeze()

		// 测试 Clone 方法
		cloned := snapshot.Clone()
		assert.NotEqual(t, snapshot.SnapshotId, cloned.SnapshotId)
		// assert.Equal(t, snapshot.CostfuncCfg, cloned.CostfuncCfg)
		assert.NotSame(t, snapshot.AllShards, cloned.AllShards)
		assert.NotSame(t, snapshot.AllWorkers, cloned.AllWorkers)
		assert.NotSame(t, snapshot.AllAssignments, cloned.AllAssignments)
	})

	// 测试分配和取消分配功能
	t.Run("AssignAndUnassign", func(t *testing.T) {
		cfg := config.CostfuncConfig{}
		ctx := context.Background()
		snapshot := NewSnapshot(ctx, cfg)
		snapshot.SnapshotId = SnapshotId("snap1")

		// 添加一个 worker
		workerFullId := data.NewWorkerFullId("worker1", "session1", data.ST_MEMORY)
		worker := NewWorkerSnap(workerFullId)
		snapshot.AllWorkers.Set(workerFullId, worker)

		// 添加一个 shard 和 replica
		shardId := data.ShardId("shard1")
		replicaIdx := data.ReplicaIdx(0)
		shard := NewShardSnap(shardId, 1)
		// shard.Replicas[replicaIdx] = NewReplicaSnap(shardId, replicaIdx)
		snapshot.AllShards.Set(shardId, shard)

		// 测试 Assign 方法
		assignmentId := data.AssignmentId("assign1")
		snapshot.Assign(shardId, replicaIdx, assignmentId, workerFullId, AM_Strict)

		// 验证分配是否成功
		assignmentSnap, exists := snapshot.AllAssignments.Get(assignmentId)
		assert.True(t, exists)
		assert.Equal(t, shardId, assignmentSnap.ShardId)
		assert.Equal(t, replicaIdx, assignmentSnap.ReplicaIdx)

		// 测试 Unassign 方法
		snapshot.Unassign(workerFullId, shardId, replicaIdx, assignmentId, AM_Strict)

		// 验证取消分配是否成功
		_, exists = snapshot.AllAssignments.Get(assignmentId)
		assert.False(t, exists)
	})

	// 测试链式修改
	t.Run("ChainOfChanges", func(t *testing.T) {
		cfg := config.CostfuncConfig{}
		ctx := context.Background()
		snapshot := NewSnapshot(ctx, cfg)
		snapshot.SnapshotId = SnapshotId("snap1")

		// 添加两个 worker
		worker1 := data.NewWorkerFullId("worker1", "session1", data.ST_MEMORY)
		worker2 := data.NewWorkerFullId("worker2", "session1", data.ST_MEMORY)
		worker1Snap := NewWorkerSnap(worker1)
		snapshot.AllWorkers.Set(worker1, worker1Snap)

		worker2Snap := NewWorkerSnap(worker2)
		snapshot.AllWorkers.Set(worker2, worker2Snap)

		// 添加两个 shard，每个 shard 有两个 replica
		shard1 := data.ShardId("shard1")
		shard2 := data.ShardId("shard2")
		for _, shardId := range []data.ShardId{shard1, shard2} {
			shard := NewShardSnap(shardId, 2)
			// for replicaIdx := data.ReplicaIdx(0); replicaIdx < 2; replicaIdx++ {
			// 	shard.Replicas[replicaIdx] = NewReplicaSnap(shardId, replicaIdx)
			// }
			snapshot.AllShards.Set(shardId, shard)
		}

		// 执行一系列分配和取消分配操作
		// 1. 将 shard1:replica0 分配给 worker1
		assign1 := data.AssignmentId("assign1")
		snapshot.Assign(shard1, 0, assign1, worker1, AM_Strict)

		// 验证分配状态
		worker1Snap, _ = snapshot.AllWorkers.Get(worker1)
		assert.Equal(t, assign1, worker1Snap.Assignments[shard1])

		shard1Snap, _ := snapshot.AllShards.Get(shard1)
		assert.Contains(t, shard1Snap.Replicas[0].Assignments, assign1)

		// 2. 将 shard1:replica1 分配给 worker2
		assign2 := data.AssignmentId("assign2")
		snapshot.Assign(shard1, 1, assign2, worker2, AM_Strict)

		// 验证两个分配都存在
		worker2Snap, _ = snapshot.AllWorkers.Get(worker2)
		assert.Equal(t, assign2, worker2Snap.Assignments[shard1])

		shard1Snap, _ = snapshot.AllShards.Get(shard1)
		assert.Contains(t, shard1Snap.Replicas[1].Assignments, assign2)

		// 3. 取消 worker1 的分配，并将 shard2:replica0 分配给它
		snapshot.Unassign(worker1, shard1, 0, assign1, AM_Strict)
		assign3 := data.AssignmentId("assign3")
		snapshot.Assign(shard2, 0, assign3, worker1, AM_Strict)

		// 验证状态变化
		worker1Snap, _ = snapshot.AllWorkers.Get(worker1)
		assert.NotContains(t, worker1Snap.Assignments, shard1)
		assert.Equal(t, assign3, worker1Snap.Assignments[shard2])

		shard1Snap, _ = snapshot.AllShards.Get(shard1)
		assert.NotContains(t, shard1Snap.Replicas[0].Assignments, assign1)

		shard2Snap, _ := snapshot.AllShards.Get(shard2)
		assert.Contains(t, shard2Snap.Replicas[0].Assignments, assign3)

		// 4. 克隆快照并继续修改
		// 在克隆前先冻结快照
		snapshot.Freeze()

		cloned := snapshot.Clone()
		assign4 := data.AssignmentId("assign4")
		cloned.Assign(shard2, 1, assign4, worker2, AM_Strict)

		// 验证原始快照未被修改
		_, exists := snapshot.AllAssignments.Get(assign4)
		assert.False(t, exists)

		// 验证克隆快照包含新的分配
		assign4Snap, exists := cloned.AllAssignments.Get(assign4)
		assert.True(t, exists)
		assert.Equal(t, shard2, assign4Snap.ShardId)
		assert.Equal(t, data.ReplicaIdx(1), assign4Snap.ReplicaIdx)
	})
}

func TestSnapshotErrorCases(t *testing.T) {
	cfg := config.CostfuncConfig{}
	ctx := context.Background()
	snapshot := NewSnapshot(ctx, cfg)
	snapshot.SnapshotId = SnapshotId("snap1")

	// 准备测试数据
	shardId := data.ShardId("shard1")
	replicaIdx := data.ReplicaIdx(0)
	assignmentId := data.AssignmentId("assign1")
	workerFullId := data.NewWorkerFullId("worker1", "session1", data.ST_MEMORY)

	// 测试分配不存在的 shard
	assert.Panics(t, func() {
		snapshot.Assign(shardId, replicaIdx, assignmentId, workerFullId, AM_Strict)
	})

	// 添加 shard 但不添加 replica
	shard := NewShardSnap(shardId, 0)
	snapshot.AllShards.Set(shardId, shard)

	// 测试分配不存在的 replica
	assert.Panics(t, func() {
		snapshot.Assign(shardId, replicaIdx, assignmentId, workerFullId, AM_Strict)
	})

	// 测试分配给不存在的 worker
	shard.Replicas[replicaIdx] = NewReplicaSnap(shardId, replicaIdx)
	assert.Panics(t, func() {
		snapshot.Assign(shardId, replicaIdx, assignmentId, workerFullId, AM_Strict)
	})

	// 测试取消不存在的分配
	assert.Panics(t, func() {
		snapshot.Unassign(workerFullId, shardId, replicaIdx, assignmentId, AM_Strict)
	})
}
