package core

import (
	"context"
	"testing"
	"time"

	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/etcdprov"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/shadow"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

// TestShadowState_EtcdStoreInteraction 测试ShadowState和EtcdStore的交互
func TestShadowState_EtcdStoreInteraction(t *testing.T) {
	// 创建FakeEtcdProvider和FakeEtcdStore
	fakeEtcd := etcdprov.NewFakeEtcdProvider()
	fakeStore := shadow.NewFakeEtcdStore()

	// 记录测试开始状态
	t.Logf("测试开始 - EtcdStore状态: %s", shadow.DumpStoreStats())
	t.Logf("测试开始 - EtcdProvider状态: %s", etcdprov.DumpGlobalState())

	// 同时替换EtcdProvider和EtcdStore
	etcdprov.RunWithEtcdProvider(fakeEtcd, func() {
		shadow.RunWithEtcdStore(fakeStore, func() {
			ctx := context.Background()
			t.Log("测试函数开始执行")

			// 检查Provider和Store是否正确设置
			currProvider := etcdprov.GetCurrentEtcdProvider(ctx)
			t.Logf("当前EtcdProvider: %T", currProvider)
			t.Logf("当前EtcdStore状态: %s", shadow.DumpStoreStats())

			// 创建PathManager和ShadowState
			pm := config.NewPathManager()
			shadowState := shadow.NewShadowState(ctx, pm)

			// 初始化完成标记
			shadowState.InitDone()

			// 创建测试数据
			t.Log("创建测试数据并写入ShadowState")
			shardId := data.ShardId("test-shard")

			// 手动创建ShardStateJson，而不是使用不存在的CreateTestShardStateJson函数
			shardState := &smgjson.ShardStateJson{
				ShardName: shardId,
				LameDuck:  0, // 使用0表示false
			}

			// 使用ShadowState写入数据
			t.Log("调用StoreShardState")
			shadowState.StoreShardState(shardId, shardState)

			// 等待一段时间，让异步操作有机会完成
			t.Log("等待异步操作处理...")
			time.Sleep(500 * time.Millisecond)

			// 检查EtcdStore状态
			t.Logf("FakeEtcdStore调用次数: %d", fakeStore.GetPutCalledCount())
			storeData := fakeStore.GetData()
			t.Logf("FakeEtcdStore数据条数: %d", len(storeData))
			for k, v := range storeData {
				t.Logf("  - 键: %s, 值长度: %d", k, len(v))
			}

			// 检查是否写入了分片状态
			expectedPath := pm.FmtShardStatePath(shardId)
			t.Logf("期望路径: %s", expectedPath)

			// 通过List API检查是否有数据
			items := fakeStore.List("/smg/shard_state/")
			t.Logf("通过List API找到 %d 条记录", len(items))
			for _, item := range items {
				t.Logf("  - 键: %s, 值长度: %d", item.Key, len(item.Value))
			}

			// 测试完成
			shadowState.StopAndWaitForExit(ctx)
		})
	})

	// 测试完成后的状态
	t.Logf("测试结束 - EtcdStore状态: %s", shadow.DumpStoreStats())
	t.Logf("测试结束 - EtcdProvider状态: %s", etcdprov.DumpGlobalState())
	t.Logf("测试结束 - FakeEtcdStore调用次数: %d", fakeStore.GetPutCalledCount())
}
