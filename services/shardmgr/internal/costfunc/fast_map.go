package costfunc

import "github.com/xinkaiwang/shardmanager/libs/xklib/kerror"

// FastMap 是一个高效的映射结构，支持分层存储和写时复制策略
//
// 设计特点：
//
// 1. 分层存储：
//   - baseMap 存储基础数据
//   - diffMap 存储对基础数据的修改
//   - 多个 FastMap 实例可以共享相同的 baseMap，同时维护各自的 diffMap
//
// 2. 写时复制（Copy-on-Write）：
//   - 通过 ownDiff 标志跟踪 diffMap 的所有权
//   - 只有在需要修改且不拥有所有权时才复制 diffMap
//   - Clone 方法创建共享底层数据的浅拷贝，直到需要修改时才分配新内存
//
// 3. 内存优化：
//   - 使用零值表示已删除的条目，而不是从映射中物理删除
//   - Compact 方法可以合并 diffMap 和 baseMap，清理已删除的条目
//
// 4. 类型安全：
//   - 使用泛型确保类型安全
//   - T1 必须是可比较的类型（comparable）
//   - T2 必须实现 TypeT2 接口，推荐使用指针类型以便更容易判断零值
//
// 这种设计特别适合需要频繁创建映射副本但修改较少的场景，如配置管理、状态跟踪等。

// 任何实现此接口的类型都可以用作 FastMap 的值类型
type TypeT2 interface {
	IsValueTypeT2()
}

type FastMap[T1 comparable, T2 TypeT2] struct {
	frozen  bool       // 标记当前实例是否已冻结，冻结后不允许修改
	ownDiff bool       // 标记当前实例是否拥有 diffMap 的所有权
	diffMap map[T1]*T2 // 存储对基础映射的修改，使用*T2作为值类型
	baseMap map[T1]*T2 // 存储基础数据，使用*T2作为值类型
}

// isZeroValue 检查值是否为零值
// 对于指针类型，这将检查是否为nil
func (fm *FastMap[T1, T2]) isZeroValue(v *T2) bool {
	return v == nil
}

// NewFastMap 创建并返回一个新的 FastMap 实例
func NewFastMap[T1 comparable, T2 TypeT2]() *FastMap[T1, T2] {
	return &FastMap[T1, T2]{
		frozen:  false,
		ownDiff: true,
		diffMap: make(map[T1]*T2),
		baseMap: make(map[T1]*T2),
	}
}

func (fm *FastMap[T1, T2]) Freeze() {
	fm.frozen = true
}

// Clone 创建 FastMap 的浅拷贝，共享底层 map
func (fm *FastMap[T1, T2]) Clone() *FastMap[T1, T2] {
	if !fm.frozen {
		ke := kerror.Create("FastMapNotFrozen", "FastMap not frozen")
		panic(ke)
	}
	return &FastMap[T1, T2]{
		ownDiff: false,
		diffMap: fm.diffMap,
		baseMap: fm.baseMap,
	}
}

// VisitAll 对映射中的每个非零值条目应用访问者函数
func (fm *FastMap[T1, T2]) VisitAll(visitor func(T1, *T2)) {
	// 首先访问 baseMap 中的条目
	for k, v := range fm.baseMap {
		// 检查该键是否在 diffMap 中被覆盖
		if diffV, ok := fm.diffMap[k]; ok {
			// 如果在 diffMap 中值不为零值，则使用 diffMap 中的值
			if !fm.isZeroValue(diffV) {
				visitor(k, diffV)
			}
			// 如果在 diffMap 中值为零值，则跳过（表示该条目已被删除）
		} else if !fm.isZeroValue(v) {
			// 如果键不在 diffMap 中且值不为零值，则使用 baseMap 中的值
			visitor(k, v)
		}
	}

	// 然后访问仅在 diffMap 中存在的条目
	for k, v := range fm.diffMap {
		// 跳过已在 baseMap 中访问过的键和值为零值的条目
		if _, ok := fm.baseMap[k]; !ok && !fm.isZeroValue(v) {
			visitor(k, v)
		}
	}
}

// Set 设置键值对，如果需要会创建 diffMap 的副本
func (fm *FastMap[T1, T2]) Set(key T1, value *T2) {
	if fm.frozen {
		ke := kerror.Create("FastMapFrozen", "FastMap is frozen")
		panic(ke)
	}
	// 如果不拥有 diffMap 的所有权，需要先创建副本
	if !fm.ownDiff {
		newDiff := make(map[T1]*T2, len(fm.diffMap))
		for k, v := range fm.diffMap {
			newDiff[k] = v
		}
		fm.diffMap = newDiff
		fm.ownDiff = true
	}
	// 设置键值对，直接存储指针
	fm.diffMap[key] = value
}

// Get 获取指定键的值，如果键不存在或值为零值则返回 nil 和 false
func (fm *FastMap[T1, T2]) Get(key T1) (*T2, bool) {
	// 首先检查 diffMap
	if v, ok := fm.diffMap[key]; ok {
		// 如果在 diffMap 中找到且值不为零值，则返回该值
		if !fm.isZeroValue(v) {
			return v, true
		}
		// 如果值为零值，表示该键已被删除，返回 nil 和 false
		return nil, false
	}

	// 如果不在 diffMap 中，检查 baseMap
	if v, ok := fm.baseMap[key]; ok {
		// 如果在 baseMap 中找到且值不为零值，则返回该值
		if !fm.isZeroValue(v) {
			return v, true
		}
	}

	// 键不存在或值为零值，返回 nil 和 false
	return nil, false
}

// Delete 删除键，如果需要会创建 diffMap 的副本
func (fm *FastMap[T1, T2]) Delete(key T1) {
	if fm.frozen {
		ke := kerror.Create("FastMapFrozen", "FastMap is frozen")
		panic(ke)
	}
	// 如果不拥有 diffMap 的所有权，需要先创建副本
	if !fm.ownDiff {
		newDiff := make(map[T1]*T2, len(fm.diffMap))
		for k, v := range fm.diffMap {
			newDiff[k] = v
		}
		fm.diffMap = newDiff
		fm.ownDiff = true
	}
	// 在 diffMap 中设置零值，表示删除
	fm.diffMap[key] = nil
}

// Compact 创建一个新的 FastMap，合并 baseMap 和 diffMap，清理已删除的条目
func (fm *FastMap[T1, T2]) Compact() *FastMap[T1, T2] {
	result := &FastMap[T1, T2]{
		ownDiff: true,
		diffMap: make(map[T1]*T2),
		baseMap: make(map[T1]*T2),
	}

	// 复制 baseMap 中的非零值条目
	for k, v := range fm.baseMap {
		if !fm.isZeroValue(v) {
			valueCopy := *v
			result.baseMap[k] = &valueCopy
		}
	}

	// 应用 diffMap 中的修改
	for k, v := range fm.diffMap {
		if !fm.isZeroValue(v) {
			valueCopy := *v
			result.baseMap[k] = &valueCopy
		} else {
			// 如果值为零值，确保从 baseMap 中删除该键
			delete(result.baseMap, k)
		}
	}

	return result
}
