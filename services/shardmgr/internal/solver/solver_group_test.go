package solver

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
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
	mu     sync.RWMutex // 保护对 config 的访问
	config config.SolverConfig
}

func (mcp *mockConfigProvider) SetConfig(cfg *smgjson.SolverConfigJson) {
	mcp.mu.Lock()
	defer mcp.mu.Unlock()
	mcp.config = config.SolverConfigJsonToConfig(cfg)
}

func (mcp *mockConfigProvider) GetByName(solverType SolverType) *config.BaseSolverConfig {
	mcp.mu.RLock()
	defer mcp.mu.RUnlock()

	switch solverType {
	case ST_SoftSolver:
		return &mcp.config.SoftSolverConfig
	case ST_AssignSolver:
		return &mcp.config.AssignSolverConfig
	case ST_UnassignSolver:
		return &mcp.config.UnassignSolverConfig
	}
	return nil
}

func (mcp *mockConfigProvider) GetSoftSolverConfig() *config.BaseSolverConfig {
	mcp.mu.RLock()
	defer mcp.mu.RUnlock()
	return &mcp.config.SoftSolverConfig
}

func (mcp *mockConfigProvider) GetAssignSolverConfig() *config.BaseSolverConfig {
	mcp.mu.RLock()
	defer mcp.mu.RUnlock()
	return &mcp.config.AssignSolverConfig
}

func (mcp *mockConfigProvider) GetUnassignSolverConfig() *config.BaseSolverConfig {
	mcp.mu.RLock()
	defer mcp.mu.RUnlock()
	return &mcp.config.UnassignSolverConfig
}

// FakeTime style test
// 这个测试用例用于验证 SolverGroup 的基本功能是否正常工作。具体测试场景如下：
//
// 1. **初始设置**：
//   - 创建一个基本的 costfunc.Snapshot 实例，包含成本函数配置
//   - 设置跟踪提案的数据结构和回调函数，用于收集 Solver 生成的提案
//
// 2. **单一 Solver 配置**：
//   - 创建并添加一个单一类型的 Solver（ST_SoftSolver）到 SolverGroup
//   - 配置 Solver 的基本参数：
//   - 启用 SoftSolver
//   - 每分钟运行 60 次（RunPerMinute = 60）
//   - 每次运行探索 1 个提案（ExplorePerRun = 1）
//
// 3. **运行与等待**：
//   - 让 Solver 运行一段时间（3秒）
//   - 在这段时间内，Solver 预期会生成一些提案
//
// 4. **基本验证**：
//   - 验证是否生成了至少一个提案，确保提案收集机制正常
//   - 验证 Solver 的 FindProposal 方法是否被调用了至少一次，确保 Solver 的运行机制正常
//
// 这个测试用例的主要目的是验证 SolverGroup 的基本功能是否正确，包括：
// - SolverGroup 能否正确初始化并添加 Solver
// - Solver 能否正确配置并运行
// - Solver 能否生成提案
// - SolverGroup 能否正确收集 Solver 生成的提案
func TestSolverGroup_Basic(t *testing.T) {
	ctx := context.Background()
	logger := klogging.NewLogrusLogger(ctx).SetConfig(ctx, "debug", "text")
	klogging.SetDefaultLogger(logger)

	// 创建 FakeTimeProvider 实例，用于控制时间流逝
	fakeTime := kcommon.NewFakeTimeProvider(1234500000000)

	// 使用 FakeTimeProvider 运行测试
	kcommon.RunWithTimeProvider(fakeTime, func() {
		// 创建一个基本的快照
		costfuncCfg := config.CostfuncConfig{
			ShardCountCostEnable: true,
			ShardCountCostNorm:   2,
			WorkerMaxAssignments: 2,
		}
		snapshot := costfunc.NewSnapshot(ctx, costfuncCfg)

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
			SoftSolverConfig: &smgjson.BaseSolverConfigJson{
				SoftSolverEnabled: func() *bool { v := true; return &v }(),
				RunPerMinute:      func() *int32 { v := int32(60); return &v }(),
				ExplorePerRun:     func() *int32 { v := int32(1); return &v }(),
			},
		})

		RunWithSolverConfigProvider(mockProvider, func() {
			// 使用 VirtualTimeForward 前进虚拟时间，替代真实等待
			// 前进3000毫秒（3秒），等同于原测试中的 time.Sleep(3 * time.Second)
			fakeTime.VirtualTimeForward(ctx, 3000)

			// 验证是否生成了提案
			proposalMu.Lock()
			proposalCount := len(receivedProposals)
			proposalMu.Unlock()

			assert.Greater(t, proposalCount, 0, "应该生成了至少一个提案")
			assert.Greater(t, int(mockSolver.GetProposalCount()), 0, "Solver 应该被调用了至少一次")

			// 记录详细的测试结果，便于分析
			t.Logf("生成的提案数量: %d", proposalCount)
			t.Logf("Solver 被调用次数: %d", mockSolver.GetProposalCount())
		})
	})
}

// FakeTime style test
// 这个测试用例主要验证 SolverGroup 能够同时管理和运行多种类型的 Solver，并能正确地处理它们产生的提案。具体测试场景如下：

// 1. **初始设置**：
//    - 创建一个基本的 costfunc.Snapshot 实例，包含成本函数配置
//    - 设置一个回调函数，用于按照 Solver 类型分类收集提案

// 2. **多 Solver 类型同时运行**：
//    - 创建并添加三种不同类型的 Solver 到 SolverGroup：
//      - ST_SoftSolver（软求解器）
//      - ST_AssignSolver（分配求解器）
//      - ST_UnassignSolver（取消分配求解器）

// 3. **统一配置**：
//    - 为所有三种 Solver 配置相同的运行参数：
//      - 每分钟运行 60 次（RunPerMinute）
//      - 所有 Solver 都启用

// 4. **并发执行**：
//    - 让所有 Solver 同时运行一段时间（3秒）
//    - 三种类型的 Solver 会并行生成提案

// 5. **结果验证**：
//    - 验证每种类型的 Solver 都生成了至少一个提案
//    - 验证每种类型的 Solver 的 FindProposal 方法都被调用了至少一次
//    - 确保不同类型的提案被正确归类和记录

// 这个测试用例的目的是验证 SolverGroup 作为一个调度器能够正确地管理多种不同类型的 Solver，并能并行运行它们，收集它们产生的提案。这对于复杂系统中需要多种优化策略协同工作的场景非常重要，比如同时需要软状态优化、资源分配和资源释放的系统。

func TestSolverGroup_MultiSolverTypes(t *testing.T) {
	ctx := context.Background()
	logger := klogging.NewLogrusLogger(ctx).SetConfig(ctx, "debug", "text")
	klogging.SetDefaultLogger(logger)

	// 创建 FakeTimeProvider 实例，用于控制时间流逝
	fakeTime := kcommon.NewFakeTimeProvider(1234500000000)

	// 使用 FakeTimeProvider 运行测试
	kcommon.RunWithTimeProvider(fakeTime, func() {
		// 创建一个基本的快照
		costfuncCfg := config.CostfuncConfig{
			ShardCountCostEnable: true,
			ShardCountCostNorm:   2,
			WorkerMaxAssignments: 2,
		}
		snapshot := costfunc.NewSnapshot(ctx, costfuncCfg)

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
			SoftSolverConfig: &smgjson.BaseSolverConfigJson{
				SoftSolverEnabled: func() *bool { v := true; return &v }(),
				RunPerMinute:      func() *int32 { v := int32(60); return &v }(),
				ExplorePerRun:     func() *int32 { v := int32(1); return &v }(),
			},
			AssignSolverConfig: &smgjson.BaseSolverConfigJson{
				SoftSolverEnabled: func() *bool { v := true; return &v }(),
				RunPerMinute:      func() *int32 { v := int32(60); return &v }(),
			},
			UnassignSolverConfig: &smgjson.BaseSolverConfigJson{
				SoftSolverEnabled: func() *bool { v := true; return &v }(),
				RunPerMinute:      func() *int32 { v := int32(60); return &v }(),
			},
		})

		RunWithSolverConfigProvider(mockProvider, func() {
			// 使用 VirtualTimeForward 前进虚拟时间，替代真实等待
			// 前进3000毫秒（3秒），等同于原测试中的 time.Sleep(3 * time.Second)
			fakeTime.VirtualTimeForward(ctx, 3000)

			// 验证每个 solver 是否生成了提案
			proposalMu.Lock()
			defer proposalMu.Unlock()

			// 记录总提案数和每种类型的提案数
			totalProposals := 0
			proposalCounts := make(map[SolverType]int)

			for solverType, solver := range mockSolvers {
				proposalCount := len(proposalsByType[string(solverType)])
				proposalCounts[solverType] = proposalCount
				totalProposals += proposalCount

				// 验证每种类型的 solver 都生成了至少一个提案
				assert.Greater(t, proposalCount, 0, fmt.Sprintf("Solver %s 应该生成了至少一个提案", solverType))

				// 验证每种类型的 solver 的 FindProposal 方法都被调用了至少一次
				assert.Greater(t, int(solver.GetProposalCount()), 0, fmt.Sprintf("Solver %s 应该被调用了至少一次", solverType))
			}

			// 记录详细的测试结果，便于分析
			t.Logf("总共生成的提案数: %d", totalProposals)
			for solverType, count := range proposalCounts {
				t.Logf("%s 生成的提案数: %d (占比 %.2f%%)",
					solverType,
					count,
					float64(count)/float64(totalProposals)*100)
			}
		})
	})
}

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
		costfuncCfg := config.CostfuncConfig{
			ShardCountCostEnable: true,
			ShardCountCostNorm:   2,
			WorkerMaxAssignments: 2,
		}
		snapshot := costfunc.NewSnapshot(ctx, costfuncCfg)
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
				SoftSolverConfig: &smgjson.BaseSolverConfigJson{
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
				SoftSolverConfig: &smgjson.BaseSolverConfigJson{
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
