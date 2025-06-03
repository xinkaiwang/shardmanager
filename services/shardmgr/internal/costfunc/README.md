# costfunc - 损失函数

一个用于系统资源分配优化的成本函数框架，设计用于分布式系统中的分片管理和任务分配场景。

## 核心概念

- **快照 (Snapshot)**: 快照 可以冻结成不可变. 
- **成本 (Cost)**: 评分，包括硬约束和软优化目标 (每一个 snapshot 都可以计算出一个当前的 cost)
- **操作 (Move)**: 变更 (move 可以 apply 在一个 snapshot 上)
- **提案 (Proposal)**: 提案 (Proposal 里面包含一个 Move, 并且还付有此提案的预期收益) （注意，预期收益总是要基于一个具体的快照， 因为快照不同， 同一个提案的收益可以大相径庭）

## 主要API

### 创建
```go
// 创建系统状态快照
snapshot := costfunc.NewSnapshot(ctx, costfuncCfg)

// 添加一个 shard 和 replica
shardId := data.ShardId("shard1")
replicaIdx := data.ReplicaIdx(0)
shard := NewShardSnap(shardId)
shard.Replicas[replicaIdx] = NewReplicaSnap(shardId, replicaIdx)
snapshot.AllShards.Set(shardId, shard)

// 添加一个 worker
workerFullId := data.WorkerFullId{
    WorkerId:  data.WorkerId("worker1"),
    SessionId: data.SessionId("session1"),
}
worker := NewWorkerSnap(workerFullId)
snapshot.AllWorkers.Set(workerFullId, worker)

// 计算快照的成本
cost := snapshot.GetCost(ctx)
```

### 更改

```go
// once freeze, the snapshot is immutable.
snapshot.Freeze()

// 克隆快照(写时复制)
newSnapshot := snapshot.Clone() // if you want to change it, you may make updates on new clone snapshots

// apply a move
Proposal.Move.Apply(newSnapshot)

cost := newSnapshot.GetCost(ctx)
```

### 计算变更收益

```go
// 计算 old 状态成本
oldCost := snapshot.GetCost(ctx)

newSnapshot := snapshot.Clone()
Proposal.Move.Apply(newSnapshot)
newCost := newSnapshot.GetCost(ctx)

// 计算变更收益 (positive gain means good)
gain := oldCost.Substract(newCost)
```

## 性能优化

- `FastMap`提供高效的映射操作，特别针对频繁克隆和小量修改的场景
- 系统使用写时复制(Copy-on-Write)策略，减少内存占用
