package costfunc

import (
	"reflect"
	"testing"
)

// 用于基准测试的测试值类型
type benchZeroValue struct {
	data string
}

func (bv benchZeroValue) IsValueTypeT2() {}

func (tv benchZeroValue) CompareWith(other TypeT2) []string {
	oth := other.(*benchZeroValue)
	if tv.data != oth.data {
		return []string{tv.data, oth.data}
	}
	return nil
}

// 使用 reflect.ValueOf(v).IsZero() 的实现
func isZeroValueUsingIsZero[T TypeT2](v T) bool {
	return reflect.ValueOf(v).IsZero()
}

// 使用 reflect.ValueOf(v).IsNil() 的实现（当前实现）
func isZeroValueUsingIsNil[T TypeT2](v *T) bool {
	return v == nil
}

// 使用直接类型断言的实现（仅适用于已知类型）
func isZeroValueUsingTypeAssertion(v interface{}) bool {
	if v == nil {
		return true
	}

	switch val := v.(type) {
	case *benchZeroValue:
		return val == nil
	default:
		return false
	}
}

// 比较不同 isZeroValue 实现的性能
func BenchmarkIsZeroValueImplementations(b *testing.B) {
	// 测试 nil 值
	b.Run("Nil", func(b *testing.B) {
		var v *benchZeroValue = nil

		b.Run("Current_IsNil", func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = isZeroValueUsingIsNil(v)
			}
		})

		b.Run("Old_IsZero", func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = isZeroValueUsingIsZero(v)
			}
		})

		b.Run("TypeAssertion", func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = isZeroValueUsingTypeAssertion(v)
			}
		})

		b.Run("DirectNilCheck", func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = v == nil
			}
		})
	})

	// 测试非 nil 值
	b.Run("NonNil", func(b *testing.B) {
		v := &benchZeroValue{data: "test"}

		b.Run("Current_IsNil", func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = isZeroValueUsingIsNil(v)
			}
		})

		b.Run("Old_IsZero", func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = isZeroValueUsingIsZero(v)
			}
		})

		b.Run("TypeAssertion", func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = isZeroValueUsingTypeAssertion(v)
			}
		})

		b.Run("DirectNilCheck", func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = v == nil
			}
		})
	})
}

// 测试 isZeroValue 在 FastMap 操作中的影响
func BenchmarkIsZeroValueInFastMapOperations(b *testing.B) {
	// 创建测试数据
	size := 1000
	fm := NewFastMap[string, benchZeroValue]()
	for i := 0; i < size; i++ {
		key := "key-" + string(rune(i))
		fm.Set(key, &benchZeroValue{data: "value-" + string(rune(i))})
	}

	// 测试 Get 操作
	b.Run("Get", func(b *testing.B) {
		keys := make([]string, size)
		for i := 0; i < size; i++ {
			keys[i] = "key-" + string(rune(i))
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := keys[i%size]
			_, _ = fm.Get(key)
		}
	})

	// 测试 Delete 操作
	b.Run("Delete", func(b *testing.B) {
		// 创建一个新的 FastMap 用于删除操作
		deleteFm := NewFastMap[string, benchZeroValue]()
		for i := 0; i < size; i++ {
			key := "delete-key-" + string(rune(i))
			deleteFm.Set(key, &benchZeroValue{data: "value-" + string(rune(i))})
		}

		keys := make([]string, size)
		for i := 0; i < size; i++ {
			keys[i] = "delete-key-" + string(rune(i))
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// 重新创建 FastMap 以避免删除所有键
			if i%size == 0 {
				deleteFm = NewFastMap[string, benchZeroValue]()
				for j := 0; j < size; j++ {
					key := "delete-key-" + string(rune(j))
					deleteFm.Set(key, &benchZeroValue{data: "value-" + string(rune(j))})
				}
			}
			key := keys[i%size]
			deleteFm.Delete(key)
		}
	})

	// 测试 VisitAll 操作
	b.Run("VisitAll", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			count := 0
			fm.VisitAll(func(key string, value *benchZeroValue) {
				count++
			})
		}
	})
}

// 测试 isZeroValue 在高频调用场景下的性能
func BenchmarkIsZeroValueHighFrequency(b *testing.B) {
	// 创建测试数据
	values := make([]*benchZeroValue, 1000)
	for i := 0; i < 1000; i++ {
		if i%2 == 0 {
			values[i] = &benchZeroValue{data: "test"}
		} else {
			values[i] = nil
		}
	}

	b.Run("Current_IsNil", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for j := 0; j < 1000; j++ {
				_ = isZeroValueUsingIsNil(values[j])
			}
		}
	})

	b.Run("Old_IsZero", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for j := 0; j < 1000; j++ {
				_ = isZeroValueUsingIsZero(values[j])
			}
		}
	})

	b.Run("TypeAssertion", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for j := 0; j < 1000; j++ {
				_ = isZeroValueUsingTypeAssertion(values[j])
			}
		}
	})

	b.Run("DirectNilCheck", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for j := 0; j < 1000; j++ {
				_ = values[j] == nil
			}
		}
	})
}
