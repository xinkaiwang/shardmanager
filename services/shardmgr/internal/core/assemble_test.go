package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

// implements solver.SnapshotListener
type MySnapshotListener struct {
	snapshot    *costfunc.Snapshot
	updateCount int
}

func (l *MySnapshotListener) OnSnapshot(snapshot *costfunc.Snapshot) {
	l.snapshot = snapshot
	l.updateCount++
}

func TestAssembleFakeSolver(t *testing.T) {
	ctx := context.Background()
	klogging.SetDefaultLogger(klogging.NewLogrusLogger(ctx).SetConfig(ctx, "debug", "text"))

	// 配置测试环境
	setup := NewFakeTimeTestSetup(t)
	setup.SetupBasicConfig(ctx)
	t.Logf("测试环境已配置")

	snapshotListener := &MySnapshotListener{}
	fn := func() {
		// Step 1: 创建 ServiceState
		ss := AssembleSsWithShadowState(ctx, "TestAssembleFakeSolver")
		ss.SolverGroup = snapshotListener
		t.Logf("ServiceState已创建: %s", ss.Name)

		// Step 2: 创建 worker-1 eph
		workerFullId, _ := ftCreateAndSetWorkerEph(t, ss, setup, "worker-1", "session-1", "localhost:8080")

		{
			// 等待worker state创建
			waitSucc, elapsedMs := WaitUntilWorkerStateEnum(t, ss, workerFullId, data.WS_Online_healthy, 1000, 10)
			assert.Equal(t, true, waitSucc, "worker state 已创建 elapsedMs=%d", elapsedMs)
		}

		// Step 3: 创建 shardPlan and set into etcd
		// 添加三个分片 (shard-a, shard-b, shard-c)
		setup.FakeTime.VirtualTimeForward(ctx, 10)
		firstShardPlan := []string{"shard-a", "shard-b", "shard-c"}
		setShardPlan(t, setup.FakeEtcd, ctx, firstShardPlan)

		// 等待ServiceState加载分片状态
		{
			// 等待快照更新
			waitSucc, elapsedMs := WaitUntil(t, func() (bool, string) {
				if snapshotListener.snapshot == nil {
					return false, "快照未创建"
				}
				if snapshotListener.snapshot.GetCost().HardScore == 3 {
					return true, ""
				}
				return false, "快照更新数目未达预期"
			}, 1000, 10)
			assert.True(t, waitSucc, "应该能在超时前加载所有分片, 耗时=%dms", elapsedMs)
		}
		{
			// assert.Equal(t, 3, snapshotListener.updateCount, "快照更新次数应该为3") // 1:first create (empty), 2: worker added, 3: shards added
			assert.NotNil(t, snapshotListener.snapshot, "快照应该不为nil")
			cost := snapshotListener.snapshot.GetCost()
			assert.Equal(t, costfunc.Cost{HardScore: 3}, cost, "快照的成本应该为0")
		}

		// Step 4: simulate a solver move
		// assert.Equal(t, true, false, "")
	}
	// 使用 FakeTimeProvider 和模拟的 EtcdProvider/EtcdStore 运行测试
	setup.RunWith(fn)
}
