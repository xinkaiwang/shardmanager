package etcdprov

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
)

// FakeEtcdProvider 是一个纯内存实现的 EtcdProvider
type FakeEtcdProvider struct {
	// 存储键值对的主数据结构
	data map[string]*fakeKV

	// 当前修订版本号，每次修改都会递增
	currentRevision EtcdRevision

	// 用于模拟 Watch 操作的事件通道
	// key 是 watch 的路径前缀，value 是接收事件的通道列表
	watchers map[string][]chan EtcdKvItem

	// 保护并发访问的互斥锁
	mu sync.RWMutex
}

// fakeKV 表示存储中的一个键值对
type fakeKV struct {
	Value       string       // 值
	ModRevision EtcdRevision // 最后修改时的版本号
	CreateRev   EtcdRevision // 创建时的版本号
}

// NewFakeEtcdProvider 创建一个新的 FakeEtcdProvider 实例
func NewFakeEtcdProvider() *FakeEtcdProvider {
	return &FakeEtcdProvider{
		data:            make(map[string]*fakeKV),
		currentRevision: 1,
		watchers:        make(map[string][]chan EtcdKvItem),
	}
}

// Get 实现
func (f *FakeEtcdProvider) Get(ctx context.Context, key string) EtcdKvItem {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if kv, exists := f.data[key]; exists {
		return EtcdKvItem{
			Key:         key,
			Value:       kv.Value,
			ModRevision: kv.ModRevision,
		}
	}

	// 键不存在时返回空值
	return EtcdKvItem{
		Key:         key,
		Value:       "",
		ModRevision: 0,
	}
}

// Set 实现
func (f *FakeEtcdProvider) Set(ctx context.Context, key, value string) {
	klogging.Info(ctx).With("key", key).With("valueLength", len(value)).Log("FakeEtcdProviderSet", "设置键值")
	f.mu.Lock()
	defer f.mu.Unlock()

	f.currentRevision++

	kv, exists := f.data[key]
	if exists {
		kv.Value = value
		kv.ModRevision = f.currentRevision
	} else {
		f.data[key] = &fakeKV{
			Value:       value,
			ModRevision: f.currentRevision,
			CreateRev:   f.currentRevision,
		}
	}

	// 通知所有相关的 watchers
	f.notifyWatchers(key, EtcdKvItem{
		Key:         key,
		Value:       value,
		ModRevision: f.currentRevision,
	})
	klogging.Info(ctx).With("key", key).With("revision", f.currentRevision).Log("FakeEtcdProviderSet", "键值设置完成")
}

// Delete 实现
func (f *FakeEtcdProvider) Delete(ctx context.Context, key string) {
	klogging.Info(ctx).With("key", key).Log("FakeEtcdProviderDelete", "删除键")
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.data[key]; !exists {
		klogging.Info(ctx).With("key", key).Log("FakeEtcdProviderDelete", "键不存在，无需删除")
		panic(ErrKeyNotFound.With("key", key))
	}

	f.currentRevision++
	delete(f.data, key)

	// 通知 watchers 删除事件
	f.notifyWatchers(key, EtcdKvItem{
		Key:         key,
		Value:       "", // 删除事件值为空
		ModRevision: f.currentRevision,
	})
	klogging.Info(ctx).With("key", key).With("revision", f.currentRevision).Log("FakeEtcdProviderDelete", "键删除完成")
}

// List 实现
func (f *FakeEtcdProvider) List(ctx context.Context, startKey string, maxCount int) []EtcdKvItem {
	f.mu.RLock()
	defer f.mu.RUnlock()

	klogging.Info(ctx).With("startKey", startKey).With("maxCount", maxCount).With("dataCount", len(f.data)).Log("FakeEtcdProviderList", "列出键值")

	// 打印所有存储的键值，帮助调试
	for k, v := range f.data {
		klogging.Info(ctx).With("key", k).With("valueLength", len(v.Value)).With("revision", v.ModRevision).Log("FakeEtcdProviderListData", "存储的键值")
	}

	var items []EtcdKvItem
	for k, v := range f.data {
		if strings.HasPrefix(k, startKey) {
			klogging.Info(ctx).With("key", k).With("matches", true).Log("FakeEtcdProviderListCheck", "键前缀匹配")
			items = append(items, EtcdKvItem{
				Key:         k,
				Value:       v.Value,
				ModRevision: v.ModRevision,
			})
		} else {
			klogging.Info(ctx).With("key", k).With("matches", false).Log("FakeEtcdProviderListCheck", "键前缀不匹配")
		}
	}

	// 按键排序，保持结果稳定性
	sort.Slice(items, func(i, j int) bool {
		return items[i].Key < items[j].Key
	})

	// 处理 maxCount 限制
	if maxCount > 0 && len(items) > maxCount {
		items = items[:maxCount]
	}

	klogging.Info(ctx).With("startKey", startKey).With("count", len(items)).Log("FakeEtcdProviderList", "列出键值完成")
	return items
}

// LoadAllByPrefix 实现
func (f *FakeEtcdProvider) LoadAllByPrefix(ctx context.Context, pathPrefix string) ([]EtcdKvItem, EtcdRevision) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	klogging.Info(ctx).With("pathPrefix", pathPrefix).With("dataCount", len(f.data)).Log("LoadAllByPrefix", "加载所有键值")

	// 打印所有存储的键值，帮助调试
	for k, v := range f.data {
		klogging.Info(ctx).With("key", k).With("valueLength", len(v.Value)).With("revision", v.ModRevision).Log("LoadAllByPrefixData", "存储的键值")
	}

	var items []EtcdKvItem
	for k, v := range f.data {
		if strings.HasPrefix(k, pathPrefix) {
			klogging.Info(ctx).With("key", k).With("matches", true).Log("LoadAllByPrefixCheck", "键前缀匹配")
			items = append(items, EtcdKvItem{
				Key:         k,
				Value:       v.Value,
				ModRevision: v.ModRevision,
			})
		} else {
			klogging.Info(ctx).With("key", k).With("matches", false).Log("LoadAllByPrefixCheck", "键前缀不匹配")
		}
	}

	// 按键排序，保持结果稳定性
	sort.Slice(items, func(i, j int) bool {
		return items[i].Key < items[j].Key
	})

	klogging.Info(ctx).With("pathPrefix", pathPrefix).With("count", len(items)).With("revision", f.currentRevision).Log("LoadAllByPrefix", "加载完成")
	return items, f.currentRevision
}

// WatchByPrefix 实现
func (f *FakeEtcdProvider) WatchByPrefix(ctx context.Context, pathPrefix string, revision EtcdRevision) chan EtcdKvItem {
	f.mu.Lock()
	ch := make(chan EtcdKvItem, 100) // 使用缓冲通道避免阻塞
	f.watchers[pathPrefix] = append(f.watchers[pathPrefix], ch)
	f.mu.Unlock()

	// 如果指定了起始版本，发送从该版本开始的所有变更
	if revision > 0 {
		go f.sendHistoricalEvents(ctx, pathPrefix, revision, ch)
	}

	// 当上下文取消时清理 watcher
	go func() {
		<-ctx.Done()
		f.mu.Lock()
		defer f.mu.Unlock()
		f.removeWatcher(pathPrefix, ch)
		close(ch)
	}()

	return ch
}

// 辅助方法：通知 watchers
func (f *FakeEtcdProvider) notifyWatchers(key string, item EtcdKvItem) {
	for prefix, channels := range f.watchers {
		if strings.HasPrefix(key, prefix) {
			for _, ch := range channels {
				// 使用非阻塞发送避免死锁
				select {
				case ch <- item:
				default:
					// 通道已满，跳过
				}
			}
		}
	}
}

// 辅助方法：移除 watcher
func (f *FakeEtcdProvider) removeWatcher(prefix string, ch chan EtcdKvItem) {
	watchers := f.watchers[prefix]
	for i, w := range watchers {
		if w == ch {
			// 从切片中移除
			watchers = append(watchers[:i], watchers[i+1:]...)
			break
		}
	}
	if len(watchers) == 0 {
		delete(f.watchers, prefix)
	} else {
		f.watchers[prefix] = watchers
	}
}

// 辅助方法：发送历史事件
func (f *FakeEtcdProvider) sendHistoricalEvents(ctx context.Context, prefix string, revision EtcdRevision, ch chan EtcdKvItem) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	for k, v := range f.data {
		if strings.HasPrefix(k, prefix) && v.ModRevision >= revision {
			select {
			case <-ctx.Done():
				return
			case ch <- EtcdKvItem{
				Key:         k,
				Value:       v.Value,
				ModRevision: v.ModRevision,
			}:
			}
		}
	}
}

// DebugDump 用于调试，打印当前存储的所有键值对
func (f *FakeEtcdProvider) DebugDump() string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if len(f.data) == 0 {
		return "FakeEtcdProvider: empty data store"
	}

	var keys []string
	for k := range f.data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var result strings.Builder
	result.WriteString(fmt.Sprintf("FakeEtcdProvider: %d keys, current revision: %d\n", len(f.data), f.currentRevision))
	for _, k := range keys {
		v := f.data[k]
		result.WriteString(fmt.Sprintf("  %s: value_len=%d, rev=%d, create_rev=%d\n",
			k, len(v.Value), v.ModRevision, v.CreateRev))
	}
	return result.String()
}
