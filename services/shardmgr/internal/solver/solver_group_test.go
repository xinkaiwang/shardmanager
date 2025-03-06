package solver

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

// MockSolver implements Solver interface for testing
type MockSolver struct {
	solverType    SolverType
	proposalCount int32
	mu            sync.Mutex
}

func NewMockSolver(solverType SolverType) *MockSolver {
	return &MockSolver{
		solverType: solverType,
	}
}

func (ms *MockSolver) GetType() SolverType {
	return ms.solverType
}

func (ms *MockSolver) FindProposal(ctx context.Context, snapshot *costfunc.Snapshot) *costfunc.Proposal {
	ms.mu.Lock()
	ms.proposalCount++
	ms.mu.Unlock()

	return &costfunc.Proposal{
		SolverType: string(ms.solverType),
		Gain: costfunc.Gain{
			HardScore: 1,
			SoftScore: 1.0,
		},
	}
}

func (ms *MockSolver) GetProposalCount() int32 {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return ms.proposalCount
}

func (ms *MockSolver) Reset() {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.proposalCount = 0
}

// 测试用的配置提供者
type mockConfigProvider struct {
	config config.SolverConfig
}

func (mcp *mockConfigProvider) SetConfig(cfg *smgjson.SolverConfigJson) {
	mcp.config = config.SolverConfigJsonToConfig(cfg)
}

func (mcp *mockConfigProvider) GetByName(solverType SolverType) *config.BaseSolverConfig {
	switch solverType {
	case ST_SoftSolver:
		return &mcp.config.SoftSolverConfig.BaseSolverConfig
	case ST_AssignSolver:
		return &mcp.config.AssignSolverConfig.BaseSolverConfig
	case ST_UnassignSolver:
		return &mcp.config.UnassignSolverConfig.BaseSolverConfig
	}
	return nil
}

func (mcp *mockConfigProvider) GetSoftSolverConfig() *config.SoftSolverConfig {
	return &mcp.config.SoftSolverConfig
}

func (mcp *mockConfigProvider) GetAssignSolverConfig() *config.AssignSolverConfig {
	return &mcp.config.AssignSolverConfig
}

func (mcp *mockConfigProvider) GetUnassignSolverConfig() *config.UnassignSolverConfig {
	return &mcp.config.UnassignSolverConfig
}

func TestSolverGroup_Basic(t *testing.T) {
	ctx := context.Background()

	// 创建一个基本的快照
	snapshot := &costfunc.Snapshot{
		SnapshotId: costfunc.SnapshotId("test-snap"),
		CostfuncCfg: config.CostfuncConfig{
			ShardCountCostEnable: true,
			ShardCountCostNorm:   2,
			WorkerMaxAssignments: 2,
		},
		AllShards:  costfunc.NewFastMap[data.ShardId, costfunc.ShardSnap](),
		AllWorkers: costfunc.NewFastMap[data.WorkerFullId, costfunc.WorkerSnap](),
		AllAssigns: costfunc.NewFastMap[data.AssignmentId, costfunc.AssignmentSnap](),
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
	group.AddSolver(ctx, mockSolver)

	// 配置 solver
	mockProvider := &mockConfigProvider{}
	mockProvider.SetConfig(&smgjson.SolverConfigJson{
		SoftSolverConfig: &smgjson.SoftSolverConfigJson{
			SoftSolverEnabled: func() *bool { v := true; return &v }(),
			RunPerMinute:      func() *int32 { v := int32(60); return &v }(),
			ExplorePerRun:     func() *int32 { v := int32(1); return &v }(),
		},
	})

	RunWithSolverConfigProvider(mockProvider, func() {
		// 等待一段时间让 solver 运行
		time.Sleep(3 * time.Second) // 减少等待时间，因为我们已经减少了线程的睡眠时间

		// 验证是否生成了提案
		proposalMu.Lock()
		proposalCount := len(receivedProposals)
		proposalMu.Unlock()

		assert.Greater(t, proposalCount, 0, "应该生成了至少一个提案")
		assert.Greater(t, int(mockSolver.GetProposalCount()), 0, "Solver 应该被调用了至少一次")
	})
}

func TestSolverGroup_MultiSolver(t *testing.T) {
	ctx := context.Background()

	// 创建一个基本的快照
	snapshot := &costfunc.Snapshot{
		SnapshotId: costfunc.SnapshotId("test-snap"),
		CostfuncCfg: config.CostfuncConfig{
			ShardCountCostEnable: true,
			ShardCountCostNorm:   2,
			WorkerMaxAssignments: 2,
		},
		AllShards:  costfunc.NewFastMap[data.ShardId, costfunc.ShardSnap](),
		AllWorkers: costfunc.NewFastMap[data.WorkerFullId, costfunc.WorkerSnap](),
		AllAssigns: costfunc.NewFastMap[data.AssignmentId, costfunc.AssignmentSnap](),
	}

	// 跟踪每个 solver 类型收到的提案
	proposalsByType := make(map[string][]*costfunc.Proposal)
	var proposalMu sync.Mutex

	// 创建 SolverGroup
	group := NewSolverGroup(ctx, snapshot, func(proposal *costfunc.Proposal) common.EnqueueResult {
		if proposal == nil {
			return common.ER_LowGain
		}
		proposalMu.Lock()
		proposalsByType[proposal.SolverType] = append(proposalsByType[proposal.SolverType], proposal)
		proposalMu.Unlock()
		return common.ER_Enqueued
	})

	// 创建并添加所有类型的 mock solver
	mockSolvers := map[SolverType]*MockSolver{
		ST_SoftSolver:     NewMockSolver(ST_SoftSolver),
		ST_AssignSolver:   NewMockSolver(ST_AssignSolver),
		ST_UnassignSolver: NewMockSolver(ST_UnassignSolver),
	}

	for _, solver := range mockSolvers {
		group.AddSolver(ctx, solver)
	}

	// 配置所有 solver
	mockProvider := &mockConfigProvider{}
	mockProvider.SetConfig(&smgjson.SolverConfigJson{
		SoftSolverConfig: &smgjson.SoftSolverConfigJson{
			SoftSolverEnabled: func() *bool { v := true; return &v }(),
			RunPerMinute:      func() *int32 { v := int32(60); return &v }(),
			ExplorePerRun:     func() *int32 { v := int32(1); return &v }(),
		},
		AssignSolverConfig: &smgjson.AssignSolverConfigJson{
			AssignSolverEnabled: func() *bool { v := true; return &v }(),
			RunPerMinute:        func() *int32 { v := int32(60); return &v }(),
		},
		UnassignSolverConfig: &smgjson.UnassignSolverConfigJson{
			UnassignSolverEnabled: func() *bool { v := true; return &v }(),
			RunPerMinute:          func() *int32 { v := int32(60); return &v }(),
		},
	})

	RunWithSolverConfigProvider(mockProvider, func() {
		// 等待一段时间让所有 solver 运行
		time.Sleep(3 * time.Second) // 减少等待时间，因为我们已经减少了线程的睡眠时间

		// 验证每个 solver 是否生成了提案
		proposalMu.Lock()
		for solverType, solver := range mockSolvers {
			proposalCount := len(proposalsByType[string(solverType)])
			assert.Greater(t, proposalCount, 0, fmt.Sprintf("Solver %s 应该生成了至少一个提案", solverType))
			assert.Greater(t, int(solver.GetProposalCount()), 0, fmt.Sprintf("Solver %s 应该被调用了至少一次", solverType))
		}
		proposalMu.Unlock()
	})
}

func TestSolverGroup_ThreadScaling(t *testing.T) {
	ctx := context.Background()

	// 创建一个基本的快照
	snapshot := &costfunc.Snapshot{
		SnapshotId: costfunc.SnapshotId("test-snap"),
		CostfuncCfg: config.CostfuncConfig{
			ShardCountCostEnable: true,
			ShardCountCostNorm:   2,
			WorkerMaxAssignments: 2,
		},
		AllShards:  costfunc.NewFastMap[data.ShardId, costfunc.ShardSnap](),
		AllWorkers: costfunc.NewFastMap[data.WorkerFullId, costfunc.WorkerSnap](),
		AllAssigns: costfunc.NewFastMap[data.AssignmentId, costfunc.AssignmentSnap](),
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
	group.AddSolver(ctx, mockSolver)

	// 配置 solver 为低 QPM
	mockProvider := &mockConfigProvider{}
	mockProvider.SetConfig(&smgjson.SolverConfigJson{
		SoftSolverConfig: &smgjson.SoftSolverConfigJson{
			SoftSolverEnabled: func() *bool { v := true; return &v }(),
			RunPerMinute:      func() *int32 { v := int32(1); return &v }(), // 低 QPM
			ExplorePerRun:     func() *int32 { v := int32(1); return &v }(),
		},
	})

	var lowQPMProposalCount int
	RunWithSolverConfigProvider(mockProvider, func() {
		// 等待一段时间让 solver 运行
		time.Sleep(3 * time.Second) // 减少等待时间，因为我们已经减少了线程的睡眠时间

		proposalMu.Lock()
		lowQPMProposalCount = len(receivedProposals)
		proposalMu.Unlock()
	})

	// 重置提案计数
	receivedProposals = nil
	mockSolver.Reset()

	// 配置 solver 为高 QPM
	mockProvider.SetConfig(&smgjson.SolverConfigJson{
		SoftSolverConfig: &smgjson.SoftSolverConfigJson{
			SoftSolverEnabled: func() *bool { v := true; return &v }(),
			RunPerMinute:      func() *int32 { v := int32(600); return &v }(), // 高 QPM
			ExplorePerRun:     func() *int32 { v := int32(1); return &v }(),
		},
	})

	RunWithSolverConfigProvider(mockProvider, func() {
		// 等待一段时间让 solver 运行
		time.Sleep(3 * time.Second) // 减少等待时间，因为我们已经减少了线程的睡眠时间

		proposalMu.Lock()
		highQPMProposalCount := len(receivedProposals)
		proposalMu.Unlock()

		// 高 QPM 配置应该生成明显更多的提案
		assert.Greater(t, highQPMProposalCount, lowQPMProposalCount*5, "高 QPM 配置应该生成明显更多的提案")
	})
}
