package solver

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

// FakeTime style test
// 这个测试用例主要测试 SolverGroup 在不同 QPM（每分钟查询次数）配置下的线程扩展性能。具体测试场景如下：

// 1. 首先创建一个基本的快照和 SolverGroup，并添加一个 MockSolver
// 2. 测试低 QPM 配置（每分钟600次）下的性能，记录产生的提案数量
// 3. 测试高 QPM 配置（每分钟1200次）下的性能，记录产生的提案数量
// 4. 验证高 QPM 配置下生成的提案数量应该大于低 QPM 配置

// 这个测试的目的是验证 SolverGroup 能够根据配置自动扩展线程处理能力，当 QPM 提高时，系统能够相应地生成更多的提案，说明线程扩展功能正常工作。这是优化系统性能和确保处理能力能够与负载需求相匹配的重要测试。
func TestSolverGroup_ThreadScaling(t *testing.T) {
	ctx := context.Background()
	logger := klogging.NewLogrusLogger(ctx).SetConfig(ctx, "debug", "text")
	klogging.SetDefaultLogger(logger)

	// 创建 FakeTimeProvider 实例，用于控制时间流逝
	fakeTime := kcommon.NewFakeTimeProvider(1234500000000)

	// 使用 FakeTimeProvider 运行测试
	kcommon.RunWithTimeProvider(fakeTime, func() {
		// 创建一个基本的快照
		snapshot := &costfunc.Snapshot{
			SnapshotId: costfunc.SnapshotId("test-snap"),
			CostfuncCfg: config.CostfuncConfig{
				ShardCountCostEnable: true,
				ShardCountCostNorm:   2,
				WorkerMaxAssignments: 2,
			},
			AllShards:      costfunc.NewFastMap[data.ShardId, costfunc.ShardSnap](),
			AllWorkers:     costfunc.NewFastMap[data.WorkerFullId, costfunc.WorkerSnap](),
			AllAssignments: costfunc.NewFastMap[data.AssignmentId, costfunc.AssignmentSnap](),
		}

		// 跟踪收到的提案
		var receivedProposals []*costfunc.Proposal
		var proposalMu sync.Mutex

		// 创建 SolverGroup
		group := NewSolverGroup(ctx, snapshot, func(proposal *costfunc.Proposal) common.EnqueueResult {
			if proposal == nil {
				return common.ER_LowGain
			}
			proposalMu.Lock()
			receivedProposals = append(receivedProposals, proposal)
			proposalMu.Unlock()
			return common.ER_Enqueued
		})

		// 创建并添加 mock solver
		mockSolver := NewMockSolver(ST_SoftSolver)

		// 测试低 QPM 配置
		var lowQPMProposalCount int
		{
			// 配置 solver 为低 QPM
			mockProvider := &mockConfigProvider{}
			mockProvider.SetConfig(&smgjson.SolverConfigJson{
				SoftSolverConfig: &smgjson.SoftSolverConfigJson{
					SoftSolverEnabled: func() *bool { v := true; return &v }(),
					RunPerMinute:      func() *int32 { v := int32(600); return &v }(), // 低 QPM
					ExplorePerRun:     func() *int32 { v := int32(50); return &v }(),
				},
			})

			RunWithSolverConfigProvider(mockProvider, func() {
				group.AddSolver(ctx, mockSolver)

				// 使用 VirtualTimeForward 前进虚拟时间，替代真实等待
				// 前进3000毫秒（3秒），等同于原测试中的 time.Sleep(3 * time.Second)
				fakeTime.VirtualTimeForward(ctx, 3000)

				proposalMu.Lock()
				lowQPMProposalCount = len(receivedProposals)
				proposalMu.Unlock()
			})
		}

		t.Logf("低 QPM 配置下生成的提案数量: %d", lowQPMProposalCount)
		assert.Greater(t, lowQPMProposalCount, 0, "低 QPM 配置应该生成至少一个提案")

		// 重置提案计数
		receivedProposals = nil
		mockSolver.Reset()

		// 测试高 QPM 配置
		var highQPMProposalCount int
		{
			// 配置 solver 为高 QPM
			mockProvider := &mockConfigProvider{}
			mockProvider.SetConfig(&smgjson.SolverConfigJson{
				SoftSolverConfig: &smgjson.SoftSolverConfigJson{
					SoftSolverEnabled: func() *bool { v := true; return &v }(),
					RunPerMinute:      func() *int32 { v := int32(1200); return &v }(), // 高 QPM
					ExplorePerRun:     func() *int32 { v := int32(5); return &v }(),
				},
			})

			RunWithSolverConfigProvider(mockProvider, func() {
				// 使用 VirtualTimeForward 前进虚拟时间，替代真实等待
				// 前进3000毫秒（3秒），等同于原测试中的 time.Sleep(3 * time.Second)
				fakeTime.VirtualTimeForward(ctx, 3000)

				proposalMu.Lock()
				highQPMProposalCount = len(receivedProposals)
				proposalMu.Unlock()
			})
		}

		t.Logf("高 QPM 配置下生成的提案数量: %d", highQPMProposalCount)
		assert.Greater(t, highQPMProposalCount, lowQPMProposalCount, "高 QPM 配置应该生成更多提案")

		// 记录详细的测试结果，便于分析
		t.Logf("低 QPM 提案数: %d, 高 QPM 提案数: %d, 增长比例: %.2f%%",
			lowQPMProposalCount,
			highQPMProposalCount,
			float64(highQPMProposalCount-lowQPMProposalCount)/float64(lowQPMProposalCount)*100)
	})
}
