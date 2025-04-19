package costfunc

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
)

// 用于基准测试的测试值类型
type benchValue struct {
	data string
}

func (bv benchValue) IsValueTypeT2() {}

func (tv benchValue) CompareWith(other TypeT2) []string {
	oth := other.(*benchValue)
	if tv.data != oth.data {
		return []string{tv.data, oth.data}
	}
	return nil
}

// 添加全局的isZeroValue函数，用于基准测试
// 这个函数与FastMap.isZeroValue方法功能相同，但作为全局函数存在
func isZeroValue[T TypeT2](v T) bool {
	rv := reflect.ValueOf(v)

	// 检查接口值本身是否为 nil
	if !rv.IsValid() {
		return true
	}

	// 检查指针是否为 nil
	switch rv.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Chan, reflect.Func, reflect.Map, reflect.Slice:
		return rv.IsNil()
	default:
		// 对于非指针类型，使用 IsZero
		return rv.IsZero()
	}
}

// 创建一个具有指定大小的 FastMap 用于测试
func createTestFastMap(size int) *FastMap[string, benchValue] {
	fm := NewFastMap[string, benchValue]()
	for i := 0; i < size; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := &benchValue{data: fmt.Sprintf("value-%d", i)}
		fm.Set(key, value)
	}
	return fm
}

// 测试 isZeroValue 函数的性能
func BenchmarkIsZeroValue(b *testing.B) {
	// 测试 nil 值
	b.Run("Nil", func(b *testing.B) {
		var v *benchValue = nil
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = isZeroValue(v)
		}
	})

	// 测试非 nil 值
	b.Run("NonNil", func(b *testing.B) {
		v := &benchValue{data: "test"}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = isZeroValue(v)
		}
	})
}

// 测试 FastMap 的 Get 操作性能
func BenchmarkFastMapGet(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			fm := createTestFastMap(size)
			keys := make([]string, size)
			for i := 0; i < size; i++ {
				keys[i] = fmt.Sprintf("key-%d", i)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				key := keys[i%size]
				_, _ = fm.Get(key)
			}
		})
	}

	// 测试 Get 操作在 baseMap 和 diffMap 中的性能差异
	b.Run("BaseMap_vs_DiffMap", func(b *testing.B) {
		size := 1000
		fm := createTestFastMap(size)

		// 在 diffMap 中添加一些键
		for i := 0; i < size/2; i++ {
			key := fmt.Sprintf("diff-key-%d", i)
			value := &benchValue{data: fmt.Sprintf("diff-value-%d", i)}
			fm.Set(key, value)
		}

		baseKeys := make([]string, size)
		for i := 0; i < size; i++ {
			baseKeys[i] = fmt.Sprintf("key-%d", i)
		}

		diffKeys := make([]string, size/2)
		for i := 0; i < size/2; i++ {
			diffKeys[i] = fmt.Sprintf("diff-key-%d", i)
		}

		b.Run("BaseMap", func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				key := baseKeys[i%size]
				_, _ = fm.Get(key)
			}
		})

		b.Run("DiffMap", func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				key := diffKeys[i%(size/2)]
				_, _ = fm.Get(key)
			}
		})
	})
}

// 测试 FastMap 的 Set 操作性能
func BenchmarkFastMapSet(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			fm := createTestFastMap(size)
			keys := make([]string, size)
			for i := 0; i < size; i++ {
				keys[i] = fmt.Sprintf("key-%d", i)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				key := keys[i%size]
				value := &benchValue{data: fmt.Sprintf("new-value-%d", i)}
				fm.Set(key, value)
			}
		})
	}

	// 测试 Set 操作在原始 FastMap 和克隆的 FastMap 上的性能差异
	b.Run("Original_vs_Clone", func(b *testing.B) {
		size := 1000
		original := createTestFastMap(size)
		cloned := original.Clone()

		keys := make([]string, size)
		for i := 0; i < size; i++ {
			keys[i] = fmt.Sprintf("key-%d", i)
		}

		b.Run("Original", func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				key := keys[i%size]
				value := &benchValue{data: fmt.Sprintf("original-value-%d", i)}
				original.Set(key, value)
			}
		})

		b.Run("Cloned", func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				key := keys[i%size]
				value := &benchValue{data: fmt.Sprintf("cloned-value-%d", i)}
				cloned.Set(key, value)
			}
		})
	})
}

// 测试 FastMap 的 Delete 操作性能
func BenchmarkFastMapDelete(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			fm := createTestFastMap(size)
			keys := make([]string, size)
			for i := 0; i < size; i++ {
				keys[i] = fmt.Sprintf("key-%d", i)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				key := keys[i%size]
				fm.Delete(key)
			}
		})
	}

	// 测试 Delete 操作在原始 FastMap 和克隆的 FastMap 上的性能差异
	b.Run("Original_vs_Clone", func(b *testing.B) {
		size := 1000
		original := createTestFastMap(size)
		cloned := original.Clone()

		keys := make([]string, size)
		for i := 0; i < size; i++ {
			keys[i] = fmt.Sprintf("key-%d", i)
		}

		b.Run("Original", func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				key := keys[i%size]
				original.Delete(key)
			}
		})

		b.Run("Cloned", func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				key := keys[i%size]
				cloned.Delete(key)
			}
		})
	})
}

// 测试 FastMap 的 VisitAll 操作性能
func BenchmarkFastMapVisitAll(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			fm := createTestFastMap(size)

			// 在 diffMap 中添加一些值，并删除一些 baseMap 中的键
			for i := 0; i < size/2; i++ {
				key := fmt.Sprintf("diff-key-%d", i)
				value := &benchValue{data: fmt.Sprintf("diff-value-%d", i)}
				fm.Set(key, value)

				// 删除一些 baseMap 中的键
				if i%2 == 0 {
					deleteKey := fmt.Sprintf("key-%d", i)
					fm.Delete(deleteKey)
				}
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				count := 0
				fm.VisitAll(func(key string, value *benchValue) {
					count++
				})
			}
		})
	}
}

// 测试 FastMap 的 Clone 操作性能
func BenchmarkFastMapClone(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			fm := createTestFastMap(size)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = fm.Clone()
			}
		})
	}
}

// 测试 FastMap 的 Compact 操作性能
func BenchmarkFastMapCompact(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size_%d_NoDiff", size), func(b *testing.B) {
			fm := createTestFastMap(size)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = fm.Compact()
			}
		})

		b.Run(fmt.Sprintf("Size_%d_WithDiff", size), func(b *testing.B) {
			fm := createTestFastMap(size)

			// 在 diffMap 中添加一些值，并删除一些 baseMap 中的键
			for i := 0; i < size/2; i++ {
				key := fmt.Sprintf("diff-key-%d", i)
				value := &benchValue{data: fmt.Sprintf("diff-value-%d", i)}
				fm.Set(key, value)

				// 删除一些 baseMap 中的键
				if i%2 == 0 {
					deleteKey := fmt.Sprintf("key-%d", i)
					fm.Delete(deleteKey)
				}
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = fm.Compact()
			}
		})
	}
}

// 测试 FastMap 在真实场景中的性能
func BenchmarkFastMapRealWorldScenario(b *testing.B) {
	// 模拟真实场景：创建一个基础 FastMap，然后创建多个克隆并进行修改
	baseSize := 1000
	base := createTestFastMap(baseSize)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 创建克隆
		clone := base.Clone()

		// 模拟随机操作
		for j := 0; j < 100; j++ {
			randKey := fmt.Sprintf("key-%d", rand.Intn(baseSize))
			if rand.Float32() < 0.7 {
				// 70% 概率设置值
				value := &benchValue{data: fmt.Sprintf("new-value-%d", i*10+j)}
				clone.Set(randKey, value)
			} else {
				// 30% 概率删除值
				clone.Delete(randKey)
			}
		}

		// 压缩 FastMap
		_ = clone.Compact()
	}
}

// 测试 FastMap 与标准 map 的性能对比
func BenchmarkFastMapVsStandardMap(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000}

	for _, size := range sizes {
		// 准备测试数据
		keys := make([]string, size)
		for i := 0; i < size; i++ {
			keys[i] = fmt.Sprintf("key-%d", i)
		}

		// 测试 FastMap 的 Get 操作
		b.Run(fmt.Sprintf("FastMap_Get_Size_%d", size), func(b *testing.B) {
			fm := createTestFastMap(size)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				key := keys[i%size]
				_, _ = fm.Get(key)
			}
		})

		// 测试标准 map 的 Get 操作
		b.Run(fmt.Sprintf("StandardMap_Get_Size_%d", size), func(b *testing.B) {
			stdMap := make(map[string]*benchValue, size)
			for i := 0; i < size; i++ {
				key := fmt.Sprintf("key-%d", i)
				stdMap[key] = &benchValue{data: fmt.Sprintf("value-%d", i)}
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				key := keys[i%size]
				_, _ = stdMap[key]
			}
		})

		// 测试 FastMap 的 Set 操作
		b.Run(fmt.Sprintf("FastMap_Set_Size_%d", size), func(b *testing.B) {
			fastMap := createTestFastMap(size)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				key := keys[i%size]
				value := &benchValue{data: fmt.Sprintf("new-value-%d", i)}
				fastMap.Set(key, value)
			}
		})

		// 测试标准 map 的 Set 操作
		b.Run(fmt.Sprintf("StandardMap_Set_Size_%d", size), func(b *testing.B) {
			stdMap := make(map[string]*benchValue, size)
			for i := 0; i < size; i++ {
				key := fmt.Sprintf("key-%d", i)
				stdMap[key] = &benchValue{data: fmt.Sprintf("value-%d", i)}
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				key := keys[i%size]
				stdMap[key] = &benchValue{data: fmt.Sprintf("new-value-%d", i)}
			}
		})
	}
}
