package costfunc

import (
	"fmt"
	"testing"
)

// 实现 TypeT2 接口的测试结构体
type testValue struct {
	data string
}

func (tv testValue) CompareWith(other TypeT2) []string {
	oth := other.(*testValue)
	if tv.data != oth.data {
		return []string{tv.data, oth.data}
	}
	return nil
}

// IsValueTypeT2 实现 TypeT2 接口
func (tv testValue) IsValueTypeT2() {}

// NewTestValue 返回一个新的测试值实例
func NewTestValue(data string) *testValue {
	return &testValue{data: data}
}

// TestNewFastMap 测试创建新的 FastMap
func TestNewFastMap(t *testing.T) {
	fm := NewFastMap[string, testValue]()
	if fm == nil {
		t.Fatal("NewFastMap 返回了 nil")
	}
	if fm.diffMap == nil {
		t.Error("diffMap 为 nil")
	}
	if fm.baseMap == nil {
		t.Error("baseMap 为 nil")
	}
	if !fm.ownDiff {
		t.Error("ownDiff 应该为 true")
	}
}

// TestFastMapSetGet 测试设置和获取值
func TestFastMapSetGet(t *testing.T) {
	fm := NewFastMap[string, testValue]()

	// 测试设置和获取值
	key := "test-key"
	value := NewTestValue("test-data")
	fm.Set(key, value)

	// 获取存在的键
	got, ok := fm.Get(key)
	if !ok {
		t.Errorf("Get(%q) 返回 ok=false，期望 ok=true", key)
	}
	// 现在直接比较指针是否相同
	if got != value {
		t.Errorf("Get(%q) = %v, 期望 %v", key, got, value)
	}

	// 获取不存在的键
	nonExistKey := "non-exist-key"
	got, ok = fm.Get(nonExistKey)
	if ok {
		t.Errorf("Get(%q) 返回 ok=true，期望 ok=false", nonExistKey)
	}
	if got != nil {
		t.Errorf("Get(%q) = %v, 期望 nil", nonExistKey, got)
	}

	// 测试覆盖值
	newValue := NewTestValue("new-data")
	fm.Set(key, newValue)
	got, ok = fm.Get(key)
	if !ok {
		t.Errorf("Get(%q) 返回 ok=false，期望 ok=true", key)
	}
	// 现在直接比较指针是否相同
	if got != newValue {
		t.Errorf("Get(%q) = %v, 期望 %v", key, got, newValue)
	}
}

// TestFastMapDelete 测试删除键
func TestFastMapDelete(t *testing.T) {
	fm := NewFastMap[string, testValue]()

	// 设置值
	key := "test-key"
	value := NewTestValue("test-data")
	fm.Set(key, value)

	// 确认值存在
	_, ok := fm.Get(key)
	if !ok {
		t.Fatalf("设置后值不存在")
	}

	// 删除值
	fm.Delete(key)

	// 确认值已删除
	got, ok := fm.Get(key)
	if ok {
		t.Errorf("删除后 Get(%q) 返回 ok=true，期望 ok=false", key)
	}
	if got != nil {
		t.Errorf("删除后 Get(%q) = %v, 期望 nil", key, got)
	}
}

// TestFastMapClone 测试克隆功能
func TestFastMapClone(t *testing.T) {
	fm := NewFastMap[string, testValue]()

	// 设置一些初始值
	fm.Set("key1", NewTestValue("value1"))
	fm.Set("key2", NewTestValue("value2"))

	// 在克隆前冻结FastMap
	fm.Freeze()

	// 克隆 FastMap
	clone := fm.Clone()

	// 验证克隆的 FastMap 包含相同的值
	for _, key := range []string{"key1", "key2"} {
		origVal, origOk := fm.Get(key)
		cloneVal, cloneOk := clone.Get(key)

		if origOk != cloneOk {
			t.Errorf("键 %q 的存在状态不匹配: 原始=%v, 克隆=%v", key, origOk, cloneOk)
		}

		if origVal != cloneVal {
			t.Errorf("键 %q 的值不匹配: 原始=%v, 克隆=%v", key, origVal, cloneVal)
		}
	}

	// 验证克隆的 FastMap 共享相同的底层 map
	// 使用指针比较而不是直接比较 map
	diffMapPtr1 := fmt.Sprintf("%p", fm.diffMap)
	diffMapPtr2 := fmt.Sprintf("%p", clone.diffMap)
	if diffMapPtr1 != diffMapPtr2 {
		t.Error("克隆的 diffMap 不是共享的")
	}

	baseMapPtr1 := fmt.Sprintf("%p", fm.baseMap)
	baseMapPtr2 := fmt.Sprintf("%p", clone.baseMap)
	if baseMapPtr1 != baseMapPtr2 {
		t.Error("克隆的 baseMap 不是共享的")
	}

	// 验证 ownDiff 标志
	if clone.ownDiff {
		t.Error("克隆的 ownDiff 应该为 false")
	}

	// 修改克隆，验证写时复制
	clone.Set("key3", NewTestValue("value3"))

	// 验证克隆的 diffMap 已经复制
	diffMapPtr1 = fmt.Sprintf("%p", fm.diffMap)
	diffMapPtr2 = fmt.Sprintf("%p", clone.diffMap)
	if diffMapPtr1 == diffMapPtr2 {
		t.Error("修改后克隆的 diffMap 仍然共享")
	}

	// 验证原始 FastMap 没有受到影响
	if _, ok := fm.Get("key3"); ok {
		t.Error("修改克隆影响了原始 FastMap")
	}

	// 验证克隆包含新值
	if val, ok := clone.Get("key3"); !ok || val.data != "value3" {
		t.Error("克隆中没有正确设置新值")
	}
}

// TestFastMapVisitAll 测试 VisitAll 方法
func TestFastMapVisitAll(t *testing.T) {
	fm := NewFastMap[string, testValue]()

	// 设置一些值
	testData := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	for k, v := range testData {
		fm.Set(k, NewTestValue(v))
	}

	// 使用 VisitAll 收集所有键值对
	visited := make(map[string]string)
	fm.VisitAll(func(k string, v *testValue) {
		visited[k] = v.data
	})

	// 验证访问了所有键值对
	if len(visited) != len(testData) {
		t.Errorf("VisitAll 访问了 %d 个键值对，期望 %d 个", len(visited), len(testData))
	}

	for k, v := range testData {
		if visited[k] != v {
			t.Errorf("键 %q 的值不匹配: 得到=%q, 期望=%q", k, visited[k], v)
		}
	}

	// 删除一个键
	fm.Delete("key2")

	visited = make(map[string]string)
	fm.VisitAll(func(k string, v *testValue) {
		visited[k] = v.data
	})

	if len(visited) != len(testData)-1 {
		t.Errorf("删除后 VisitAll 访问了 %d 个键值对，期望 %d 个", len(visited), len(testData)-1)
	}

	if _, ok := visited["key2"]; ok {
		t.Error("VisitAll 访问了已删除的键")
	}
}

// TestFastMapCompact 测试压缩功能
func TestFastMapCompact(t *testing.T) {
	fm := NewFastMap[string, testValue]()

	// 设置一些基础数据
	baseData := map[string]string{
		"base1": "base-value1",
		"base2": "base-value2",
		"base3": "base-value3",
	}

	for k, v := range baseData {
		fm.baseMap[k] = NewTestValue(v)
	}

	// 在 diffMap 中设置一些值
	diffData := map[string]string{
		"diff1": "diff-value1",
		"base1": "modified-value1", // 修改 baseMap 中的值
	}

	for k, v := range diffData {
		fm.Set(k, NewTestValue(v))
	}

	// 删除一个 baseMap 中的键
	fm.Delete("base3")

	// 压缩 FastMap
	compacted := fm.Compact()

	// 验证压缩后的 FastMap 包含正确的值
	expectedData := map[string]string{
		"base1": "modified-value1", // 修改后的值
		"base2": "base-value2",     // 未修改的值
		"diff1": "diff-value1",     // 新增的值
		// base3 已被删除
	}

	// 验证所有期望的键值对都存在
	for k, expectedValue := range expectedData {
		if v, ok := compacted.baseMap[k]; !ok {
			t.Errorf("键 %q 在压缩后不存在", k)
		} else if v.data != expectedValue {
			t.Errorf("键 %q 的值不匹配: 得到=%q, 期望=%q", k, v.data, expectedValue)
		}
	}

	// 验证删除的键不存在
	if _, ok := compacted.baseMap["base3"]; ok {
		t.Error("已删除的键在压缩后仍然存在")
	}

	// 验证 diffMap 为空
	if len(compacted.diffMap) != 0 {
		t.Errorf("压缩后 diffMap 不为空，包含 %d 个条目", len(compacted.diffMap))
	}

	// 验证 ownDiff 标志
	if !compacted.ownDiff {
		t.Error("压缩后 ownDiff 应该为 true")
	}
}

// TestFastMapCombined 测试组合操作
func TestFastMapCombined(t *testing.T) {
	fm := NewFastMap[string, testValue]()

	// 设置一些初始值
	initialData := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	for k, v := range initialData {
		fm.Set(k, NewTestValue(v))
	}

	// 在克隆前修改原始 FastMap
	newValue1 := NewTestValue("modified1")
	fm.Set("key1", newValue1)
	fm.Delete("key2")
	newValue4 := NewTestValue("value4")
	fm.Set("key4", newValue4)

	// 在克隆前冻结FastMap
	fm.Freeze()

	// 克隆 FastMap
	clone := fm.Clone()

	// 验证克隆包含原始 FastMap 的所有修改
	if v, ok := clone.Get("key1"); !ok || v != newValue1 {
		t.Error("克隆没有包含原始 FastMap 的修改")
	}
	if _, ok := clone.Get("key2"); ok {
		t.Error("克隆包含了原始 FastMap 中已删除的键")
	}
	if v, ok := clone.Get("key4"); !ok || v != newValue4 {
		t.Error("克隆没有包含原始 FastMap 的新增键")
	}

	// 验证原始 FastMap 已被冻结，尝试修改应该会导致异常
	defer func() {
		if r := recover(); r == nil {
			t.Error("修改已冻结的 FastMap 应该会导致异常")
		}
	}()
	fm.Set("key5", NewTestValue("value5"))

	// 压缩克隆的 FastMap
	compacted := clone.Compact()

	// 验证压缩后的 FastMap 包含所有预期的键值对
	expectedKeys := map[string]bool{
		"key1": true,
		"key3": true,
		"key4": true,
	}

	compacted.VisitAll(func(k string, v *testValue) {
		if !expectedKeys[k] {
			t.Errorf("压缩后的 FastMap 包含意外的键: %s", k)
		}
		delete(expectedKeys, k)
	})

	if len(expectedKeys) > 0 {
		t.Errorf("压缩后的 FastMap 缺少预期的键: %v", expectedKeys)
	}

	// 在克隆上进行修改
	newValue5 := NewTestValue("value5")
	clone.Set("key5", newValue5)

	// 验证克隆可以被修改
	if v, ok := clone.Get("key5"); !ok || v != newValue5 {
		t.Error("无法修改克隆的 FastMap")
	}
}

// TestFastMapDeleteWithClone 测试删除和克隆的组合操作
func TestFastMapDeleteWithClone(t *testing.T) {
	fm := NewFastMap[string, testValue]()

	// 设置一些初始值
	fm.Set("key1", NewTestValue("value1"))
	fm.Set("key2", NewTestValue("value2"))

	// 在克隆前冻结FastMap
	fm.Freeze()

	// 克隆 FastMap
	clone := fm.Clone()

	// 在克隆中删除一个键
	clone.Delete("key1")

	// 验证原始 FastMap 不受影响
	if _, ok := fm.Get("key1"); !ok {
		t.Error("克隆中的删除影响了原始 FastMap")
	}

	// 验证克隆中的键已被删除
	if _, ok := clone.Get("key1"); ok {
		t.Error("克隆中的键未被正确删除")
	}

	// 验证 diffMap 不是共享的
	if fmt.Sprintf("%p", fm.diffMap) == fmt.Sprintf("%p", clone.diffMap) {
		t.Error("删除后 diffMap 仍然共享")
	}
}

// TestFastMapVisitAllEdgeCases 测试 VisitAll 的边缘情况
func TestFastMapVisitAllEdgeCases(t *testing.T) {
	fm := NewFastMap[string, testValue]()

	// 在 baseMap 中设置一些值，包括零值
	fm.baseMap["base1"] = NewTestValue("base-value1")
	fm.baseMap["base2"] = NewTestValue("base-value2")
	var zeroValue *testValue = nil
	fm.baseMap["base-zero"] = zeroValue // 零值

	// 在 diffMap 中设置一些值，包括零值
	fm.Set("diff1", NewTestValue("diff-value1"))
	fm.Set("diff2", NewTestValue("diff-value2"))
	fm.Set("diff-zero", nil) // 零值

	// 使用 VisitAll 收集所有键值对
	visited := make(map[string]bool)
	fm.VisitAll(func(k string, v *testValue) {
		visited[k] = true
		// 验证不会访问到零值
		if v == nil {
			t.Errorf("VisitAll 访问了键 %q 的零值", k)
		}
	})

	// 验证预期的键被访问
	expectedKeys := map[string]bool{
		"base1": true,
		"base2": true,
		"diff1": true,
		"diff2": true,
		// base-zero 和 diff-zero 不应该被访问
	}

	for k := range expectedKeys {
		if !visited[k] {
			t.Errorf("VisitAll 未访问键 %q", k)
		}
	}

	for k := range visited {
		if !expectedKeys[k] {
			t.Errorf("VisitAll 访问了意外的键 %q", k)
		}
	}
}

// TestFastMapGetEdgeCases 测试 Get 的边缘情况
func TestFastMapGetEdgeCases(t *testing.T) {
	fm := NewFastMap[string, testValue]()

	// 在 baseMap 中设置一个零值
	var zeroValue *testValue = nil
	fm.baseMap["base-zero"] = zeroValue

	// 测试从 baseMap 获取零值
	got, ok := fm.Get("base-zero")
	if ok {
		t.Error("对于 baseMap 中的零值，Get 返回 ok=true，期望 ok=false")
	}
	if got != nil {
		t.Errorf("对于 baseMap 中的零值，Get 返回 %v，期望 nil", got)
	}

	// 在 diffMap 中设置一个零值（非删除标记）
	fm.diffMap["diff-zero"] = zeroValue

	// 测试从 diffMap 获取零值
	got, ok = fm.Get("diff-zero")
	if ok {
		t.Error("对于 diffMap 中的零值，Get 返回 ok=true，期望 ok=false")
	}
	if got != nil {
		t.Errorf("对于 diffMap 中的零值，Get 返回 %v，期望 nil", got)
	}
}

// TestFastMapGetComprehensive 测试 Get 的综合情况
func TestFastMapGetComprehensive(t *testing.T) {
	fm := NewFastMap[string, testValue]()

	// 测试情况1: 键不存在
	val, ok := fm.Get("non-exist")
	if ok {
		t.Error("对于不存在的键，Get 返回 ok=true，期望 ok=false")
	}
	if val != nil {
		t.Errorf("对于不存在的键，Get 返回 %v，期望 nil", val)
	}

	// 测试情况2: 键存在于 baseMap，值不为零值
	fm.baseMap["base"] = NewTestValue("base-value")
	val, ok = fm.Get("base")
	if !ok {
		t.Error("对于存在于 baseMap 的键，Get 返回 ok=false，期望 ok=true")
	}
	if val == nil || val.data != "base-value" {
		t.Errorf("对于存在于 baseMap 的键，Get 返回 %v，期望值的data为'base-value'", val)
	}

	// 测试情况3: 键存在于 baseMap，值为零值
	var zeroValue *testValue = nil
	fm.baseMap["base-zero"] = zeroValue
	val, ok = fm.Get("base-zero")
	if ok {
		t.Error("对于 baseMap 中值为零值的键，Get 返回 ok=true，期望 ok=false")
	}
	if val != nil {
		t.Errorf("对于 baseMap 中值为零值的键，Get 返回 %v，期望 nil", val)
	}

	// 测试情况4: 键存在于 diffMap，值不为零值
	fm.diffMap["diff"] = NewTestValue("diff-value")
	val, ok = fm.Get("diff")
	if !ok {
		t.Error("对于存在于 diffMap 的键，Get 返回 ok=false，期望 ok=true")
	}
	if val == nil || val.data != "diff-value" {
		t.Errorf("对于存在于 diffMap 的键，Get 返回 %v，期望值的data为'diff-value'", val)
	}

	// 测试情况5: 键存在于 diffMap，值为零值（表示删除）
	fm.diffMap["diff-zero"] = zeroValue
	val, ok = fm.Get("diff-zero")
	if ok {
		t.Error("对于 diffMap 中值为零值的键，Get 返回 ok=true，期望 ok=false")
	}
	if val != nil {
		t.Errorf("对于 diffMap 中值为零值的键，Get 返回 %v，期望 nil", val)
	}

	// 测试情况6: 键同时存在于 baseMap 和 diffMap，diffMap 中的值不为零值
	fm.baseMap["both"] = NewTestValue("base-both")
	fm.diffMap["both"] = NewTestValue("diff-both")
	val, ok = fm.Get("both")
	if !ok {
		t.Error("对于同时存在于 baseMap 和 diffMap 的键，Get 返回 ok=false，期望 ok=true")
	}
	if val == nil || val.data != "diff-both" {
		t.Errorf("对于同时存在于 baseMap 和 diffMap 的键，Get 返回 %v，期望值的data为'diff-both'", val)
	}

	// 测试情况7: 键同时存在于 baseMap 和 diffMap，diffMap 中的值为零值（表示删除）
	fm.baseMap["deleted"] = NewTestValue("base-deleted")
	fm.diffMap["deleted"] = zeroValue
	val, ok = fm.Get("deleted")
	if ok {
		t.Error("对于在 diffMap 中被删除的键，Get 返回 ok=true，期望 ok=false")
	}
	if val != nil {
		t.Errorf("对于在 diffMap 中被删除的键，Get 返回 %v，期望 nil", val)
	}
}
