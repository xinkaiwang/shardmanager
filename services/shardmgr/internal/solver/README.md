# Solver包

`solver`包提供了一组用于生成和优化资源分配方案的工具，主要用于分布式系统的资源调度优化。

## 核心组件

- `Solver`: 求解器接口，用于寻找优化的资源分配方案
- `SolverGroup`: 管理并协调多个求解器的执行
- `SolverConfigProvider`: 提供求解器的配置信息

## 使用方法

### 基本使用流程

```go
// 1. 创建资源状态快照
snapshot := GetSystemStateSnapshot()

// 2. 创建求解器组
solverGroup := solver.NewSolverGroup(ctx, snapshot, func(proposal *costfunc.Proposal) common.EnqueueResult {
    // 处理生成的优化方案
})

// 3. 添加求解器
solverGroup.AddSolver(ctx, solver.NewSoftSolver())
solverGroup.AddSolver(ctx, solver.NewAssignSolver())
solverGroup.AddSolver(ctx, solver.NewUnassignSolver())

// 4. 当系统状态变化时更新快照
solverGroup.SetCurrentSnapshot(newSnapshot)
```

### 配置求解器

通过`SolverConfigProvider`可以配置求解器的行为：

```go
// 使用默认配置提供者
provider := solver.GetCurrentSolverConfigProvider()

// 更新配置
provider.SetConfig(&smgjson.SolverConfigJson{
    SoftSolverConfig: &smgjson.SoftSolverConfigJson{
        SoftSolverEnabled: func() *bool { v := true; return &v }(),
        RunPerMinute:      func() *int32 { v := int32(60); return &v }(),
        ExplorePerRun:     func() *int32 { v := int32(10); return &v }(),
    },
    // 其他求解器配置...
})
```

## 主要求解器类型

- `SoftSolver`: 通用优化求解器，尝试通过移动分片来降低系统成本
- `AssignSolver`: 为未分配的副本寻找合适的工作节点
- `UnassignSolver`: 尝试移除不必要的分配以优化资源使用

## 配置参数

每种求解器都支持以下基本配置：

- `SolverEnabled`: 是否启用此求解器
- `RunPerMinute`: 每分钟运行次数
- `ExplorePerRun`: 每次运行探索的方案数

不同类型的求解器可能有额外的特定配置参数。 