package solver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

// 测试用的 CostProvider
type testCostProvider struct{}

// NewTestCostProvider 返回测试用的成本函数提供者
func NewTestCostProvider() *testCostProvider {
	return &testCostProvider{}
}

func (tcp *testCostProvider) CalCost(snapshot *costfunc.Snapshot) costfunc.Cost {
	// 计算每个 worker 的分片数量
	workerShardCounts := make(map[data.WorkerFullId]int)
	snapshot.AllWorkers.VisitAll(func(workerId data.WorkerFullId, worker *costfunc.WorkerSnap) {
		workerShardCounts[workerId] = len(worker.Assignments)
	})

	// 计算分片分布的标准差作为成本
	var hardScore int32
	var softScore float64
	for _, count := range workerShardCounts {
		// 理想情况下，每个 worker 应该有相同数量的分片
		// 我们使用与平均值的差异平方作为成本
		diff := float64(count) - 1.0 // 期望每个 worker 有 1 个分片
		softScore += diff * diff
	}

	return costfunc.Cost{
		HardScore: hardScore,
		SoftScore: softScore,
	}
}

func TestSoftSolver_FindProposal(t *testing.T) {
	// 设置测试上下文
	ctx := context.Background()

	// 设置 SoftSolver 配置
	cfg := config.NewSolverConfig()
	cfg.SoftSolverConfig.ExplorePerRun = 100
	GetCurrentSolverConfigProvider().OnSolverConfigChange(cfg)
	// GetCurrentSolverConfigProvider().SetConfig(&smgjson.SolverConfigJson{
	// 	SoftSolverConfig: &smgjson.BaseSolverConfigJson{
	// 		ExplorePerRun: func() *int32 { v := int32(100); return &v }(), // 增加探索次数以提高找到好方案的概率
	// 	},
	// })

	costfuncCfg := config.CostfuncConfig{
		ShardCountCostEnable: true,
		ShardCountCostNorm:   2, // 我们有两个分片
		WorkerMaxAssignments: 2, // 每个 worker 最多两个分片
	}
	// 创建一个不平衡的初始快照
	snapshot := costfunc.NewSnapshot(ctx, costfuncCfg)

	// 设置自定义成本函数提供者
	snapshot.Costfunc = NewTestCostProvider()

	// 创建两个 worker
	worker1 := data.NewWorkerFullId("worker1", "session1", data.ST_MEMORY)
	worker2 := data.NewWorkerFullId("worker2", "session1", data.ST_MEMORY)

	// worker1 有两个分片，worker2 没有分片，这是一个不平衡的状态
	snapshot.AllWorkers.Set(worker1, costfunc.NewWorkerSnap(worker1))
	snapshot.AllWorkers.Set(worker2, costfunc.NewWorkerSnap(worker2))

	// 创建两个分片，每个分片一个副本
	shard1 := data.ShardId("shard1")
	shard2 := data.ShardId("shard2")
	for _, shardId := range []data.ShardId{shard1, shard2} {
		shard := costfunc.NewShardSnap(shardId)
		shard.Replicas[0] = costfunc.NewReplicaSnap(shardId, 0)
		snapshot.AllShards.Set(shardId, shard)
	}

	// 将两个分片都分配给 worker1
	assign1 := data.AssignmentId("assign1")
	assign2 := data.AssignmentId("assign2")
	snapshot.Assign(shard1, 0, assign1, worker1)
	snapshot.Assign(shard2, 0, assign2, worker1)

	// 创建并配置求解器
	solver := NewSoftSolver()

	// 在调用求解器前冻结快照
	snapshot.Freeze()

	// 运行求解器
	proposal := solver.FindProposal(ctx, snapshot)

	// 验证求解器是否找到了更好的方案
	assert.NotNil(t, proposal, "应该找到优化方案")
	if proposal != nil {
		assert.NotNil(t, proposal.Move, "方案应该包含移动操作")
		assert.Equal(t, "SoftSolver", proposal.SolverType, "求解器类型应该正确")

		// 验证移动操作是否合理
		move := proposal.Move.(*costfunc.SimpleMove)
		assert.Equal(t, worker1, move.Src, "源worker应该是worker1")
		assert.Equal(t, worker2, move.Dst, "目标worker应该是worker2")

		// 应用移动方案
		snapshotCopy := snapshot.Clone()
		move.Apply(snapshotCopy)

		// 验证移动后的状态
		worker1Snap, _ := snapshotCopy.AllWorkers.Get(worker1)
		worker2Snap, _ := snapshotCopy.AllWorkers.Get(worker2)
		assert.Equal(t, 1, len(worker1Snap.Assignments), "worker1应该剩下一个分片")
		assert.Equal(t, 1, len(worker2Snap.Assignments), "worker2应该获得一个分片")
	}
}
