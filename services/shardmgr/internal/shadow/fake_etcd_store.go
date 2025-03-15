package shadow

import (
	"context"
	"sync"

	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
)

// FakeEtcdStore implements EtcdStore interface for testing
type FakeEtcdStore struct {
	mu        sync.RWMutex
	data      map[string]string
	putCalled int
}

// NewFakeEtcdStore creates a new FakeEtcdStore instance
func NewFakeEtcdStore() *FakeEtcdStore {
	return &FakeEtcdStore{
		data: make(map[string]string),
	}
}

func (store *FakeEtcdStore) GetByKey(key string) string {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.data[key]
}

// Put implements EtcdStore.Put
func (store *FakeEtcdStore) Put(ctx context.Context, key string, value string, name string) {
	klogging.Info(ctx).
		With("key", key).
		With("valueLength", len(value)).
		With("name", name).
		Log("FakeEtcdStorePut", "写入数据")

	// 这里添加调用栈信息，帮助调试
	klogging.Info(ctx).
		With("key", key).
		Log("FakeEtcdStorePut", "调用栈跟踪点1")

	store.mu.Lock()
	defer store.mu.Unlock()

	store.putCalled++

	klogging.Info(ctx).
		With("key", key).
		With("putCalledCount", store.putCalled).
		Log("FakeEtcdStorePut", "更新putCalled计数")

	if value == "" {
		klogging.Info(ctx).
			With("key", key).
			Log("FakeEtcdStorePut", "删除键值")
		delete(store.data, key)
	} else {
		klogging.Info(ctx).
			With("key", key).
			With("valueLength", len(value)).
			Log("FakeEtcdStorePut", "存储键值")
		store.data[key] = value
	}

	// 验证存储结果
	storedValue, exists := store.data[key]
	klogging.Info(ctx).
		With("key", key).
		With("exists", exists).
		With("storedLength", len(storedValue)).
		Log("FakeEtcdStorePut", "验证存储结果")
}

// GetPutCalledCount returns the number of times Put was called
func (store *FakeEtcdStore) GetPutCalledCount() int {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.putCalled
}

// GetData returns the current data in the store
func (store *FakeEtcdStore) GetData() map[string]string {
	store.mu.RLock()
	defer store.mu.RUnlock()

	// Make a copy to avoid concurrent modification
	result := make(map[string]string, len(store.data))
	for k, v := range store.data {
		result[k] = v
	}
	return result
}

// List returns key-value pairs with given prefix
func (store *FakeEtcdStore) List(prefix string) []KvItem {
	store.mu.RLock()
	defer store.mu.RUnlock()

	var result []KvItem
	for k, v := range store.data {
		if startsWithPrefix(k, prefix) {
			result = append(result, KvItem{
				Key:   k,
				Value: v,
			})
		}
	}
	return result
}

// Helper function to check if string starts with prefix
func startsWithPrefix(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return s[:len(prefix)] == prefix
}
