package etcdprov

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFakeEtcdProvider_BasicOperations(t *testing.T) {
	ctx := context.Background()
	provider := NewFakeEtcdProvider()

	// 测试 Set 和 Get
	t.Run("Set和Get操作", func(t *testing.T) {
		provider.Set(ctx, "/test/key1", "value1")
		item := provider.Get(ctx, "/test/key1")
		assert.Equal(t, "/test/key1", item.Key)
		assert.Equal(t, "value1", item.Value)
		assert.Equal(t, EtcdRevision(2), item.ModRevision)
	})

	// 测试获取不存在的键
	t.Run("获取不存在的键", func(t *testing.T) {
		item := provider.Get(ctx, "/test/not_exist")
		assert.Equal(t, "/test/not_exist", item.Key)
		assert.Equal(t, "", item.Value)
		assert.Equal(t, EtcdRevision(0), item.ModRevision)
	})

	// 测试更新已存在的键
	t.Run("更新已存在的键", func(t *testing.T) {
		provider.Set(ctx, "/test/key1", "value1_updated")
		item := provider.Get(ctx, "/test/key1")
		assert.Equal(t, "value1_updated", item.Value)
		assert.Equal(t, EtcdRevision(3), item.ModRevision)
	})

	// 测试删除
	t.Run("删除键", func(t *testing.T) {
		provider.Set(ctx, "/test/key2", "value2")
		provider.Delete(ctx, "/test/key2", false)
		item := provider.Get(ctx, "/test/key2")
		assert.Equal(t, "", item.Value)
		assert.Equal(t, EtcdRevision(0), item.ModRevision)
	})

	// 测试删除不存在的键
	t.Run("删除不存在的键", func(t *testing.T) {
		assert.Panics(t, func() {
			provider.Delete(ctx, "/test/not_exist", false)
		})
	})
}

func TestFakeEtcdProvider_List(t *testing.T) {
	ctx := context.Background()
	provider := NewFakeEtcdProvider()

	// 准备测试数据
	provider.Set(ctx, "/test/1", "value1")
	provider.Set(ctx, "/test/2", "value2")
	provider.Set(ctx, "/test/sub/3", "value3")
	provider.Set(ctx, "/other/4", "value4")

	// 测试列出所有键
	t.Run("列出所有以/test/开头的键", func(t *testing.T) {
		items := provider.List(ctx, "/test/", 0)
		assert.Equal(t, 3, len(items))
		assert.Equal(t, "/test/1", items[0].Key)
		assert.Equal(t, "/test/2", items[1].Key)
		assert.Equal(t, "/test/sub/3", items[2].Key)
	})

	// 测试最大数量限制
	t.Run("测试最大数量限制", func(t *testing.T) {
		items := provider.List(ctx, "/test/", 2)
		assert.Equal(t, 2, len(items))
	})
}

func TestFakeEtcdProvider_LoadAllByPrefix(t *testing.T) {
	ctx := context.Background()
	provider := NewFakeEtcdProvider()

	// 准备测试数据
	provider.Set(ctx, "/test/1", "value1")
	provider.Set(ctx, "/test/2", "value2")
	provider.Set(ctx, "/other/3", "value3")

	// 测试加载所有键
	items, revision := provider.LoadAllByPrefix(ctx, "/test/")
	assert.Equal(t, 2, len(items))
	assert.Equal(t, EtcdRevision(4), revision)
	assert.Equal(t, "/test/1", items[0].Key)
	assert.Equal(t, "/test/2", items[1].Key)
}

func TestFakeEtcdProvider_Watch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider := NewFakeEtcdProvider()

	// 启动 watcher
	ch := provider.WatchByPrefix(ctx, "/test/", 0)

	// 在另一个 goroutine 中进行修改
	go func() {
		provider.Set(ctx, "/test/key1", "value1")
		provider.Set(ctx, "/test/key2", "value2")
		provider.Delete(ctx, "/test/key1", false)
	}()

	// 验证接收到的事件
	events := make([]EtcdKvItem, 0)
	timeout := time.After(time.Second)

	// 收集3个事件（2个设置，1个删除）
	for len(events) < 3 {
		select {
		case event := <-ch:
			events = append(events, event)
		case <-timeout:
			t.Fatal("等待事件超时")
		}
	}

	// 验证事件
	assert.Equal(t, 3, len(events))
	assert.Equal(t, "/test/key1", events[0].Key)
	assert.Equal(t, "value1", events[0].Value)
	assert.Equal(t, "/test/key2", events[1].Key)
	assert.Equal(t, "value2", events[1].Value)
	assert.Equal(t, "/test/key1", events[2].Key)
	assert.Equal(t, "", events[2].Value) // 删除事件
}

func TestFakeEtcdProvider_WatchFromRevision(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	provider := NewFakeEtcdProvider()

	// 先设置一些数据
	provider.Set(ctx, "/test/key1", "value1") // revision 2
	provider.Set(ctx, "/test/key2", "value2") // revision 3

	// 从版本2开始监听
	ch := provider.WatchByPrefix(ctx, "/test/", 2)

	// 验证是否收到历史事件
	event := <-ch
	assert.Equal(t, "/test/key1", event.Key)
	assert.Equal(t, "value1", event.Value)
	assert.Equal(t, EtcdRevision(2), event.ModRevision)

	event = <-ch
	assert.Equal(t, "/test/key2", event.Key)
	assert.Equal(t, "value2", event.Value)
	assert.Equal(t, EtcdRevision(3), event.ModRevision)

	// 再设置一个新值，确认也能收到
	provider.Set(ctx, "/test/key3", "value3") // revision 4
	event = <-ch
	assert.Equal(t, "/test/key3", event.Key)
	assert.Equal(t, "value3", event.Value)
	assert.Equal(t, EtcdRevision(4), event.ModRevision)
}

func TestFakeEtcdProvider_ConcurrentOperations(t *testing.T) {
	ctx := context.Background()
	provider := NewFakeEtcdProvider()

	// 并发执行多个操作
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(i int) {
			key := fmt.Sprintf("/test/key%d", i)
			provider.Set(ctx, key, fmt.Sprintf("value%d", i))
			item := provider.Get(ctx, key)
			assert.Equal(t, fmt.Sprintf("value%d", i), item.Value)
			done <- true
		}(i)
	}

	// 等待所有操作完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证所有键都存在
	items := provider.List(ctx, "/test/", 0)
	assert.Equal(t, 10, len(items))
}
