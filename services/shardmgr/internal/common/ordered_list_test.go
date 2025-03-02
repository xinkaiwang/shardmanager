package common

import (
	"context"
	"fmt"
	"testing"
)

// MockItem 实现 OrderedListItem 接口用于测试
type MockItem struct {
	value     int           // 主要分数
	subValue  float64       // 次要分数
	signature string        // 唯一标识
	dropped   bool          // 是否被移除
	reason    EnqueueResult // 移除原因
}

func NewMockItem(value int, subValue float64, signature string) *MockItem {
	return &MockItem{
		value:     value,
		subValue:  subValue,
		signature: signature,
	}
}

func (m *MockItem) IsBetterThan(other OrderedListItem) bool {
	otherMock := other.(*MockItem)
	if m.value != otherMock.value {
		return m.value > otherMock.value
	}
	return m.subValue > otherMock.subValue
}

func (m *MockItem) Dropped(ctx context.Context, reason EnqueueResult) {
	m.dropped = true
	m.reason = reason
}

func (m *MockItem) GetSignature() string {
	return m.signature
}

func (m *MockItem) String() string {
	return fmt.Sprintf("MockItem{value: %d, subValue: %.1f, sig: %s}", m.value, m.subValue, m.signature)
}

// 测试基本的入队和排序功能
func TestOrderedList_BasicEnqueue(t *testing.T) {
	ctx := context.Background()
	list := NewOrderedList[*MockItem](3)

	items := []*MockItem{
		NewMockItem(1, 1.0, "1"),
		NewMockItem(2, 1.0, "2"),
		NewMockItem(3, 1.0, "3"),
	}

	// 测试正常入队
	for i, item := range items {
		result := list.Enqueue(ctx, item)
		if result != ER_Enqueued {
			t.Errorf("第 %d 个项目入队失败，期望 %v，得到 %v", i, ER_Enqueued, result)
		}
	}

	// 验证顺序（从最好到最差）
	for i := 0; i < len(items); i++ {
		if list.Items[i].value != 3-i {
			t.Errorf("位置 %d 的项目值错误，期望 %d，得到 %d", i, 3-i, list.Items[i].value)
		}
	}
}

// 测试重复项处理
func TestOrderedList_DuplicateHandling(t *testing.T) {
	ctx := context.Background()
	list := NewOrderedList[*MockItem](3)

	// 添加第一个项目
	item1 := NewMockItem(1, 1.0, "same")
	result := list.Enqueue(ctx, item1)
	if result != ER_Enqueued {
		t.Errorf("第一个项目入队失败，期望 %v，得到 %v", ER_Enqueued, result)
	}

	// 尝试添加具有相同签名的项目
	item2 := NewMockItem(2, 2.0, "same")
	result = list.Enqueue(ctx, item2)
	if result != ER_Duplicate {
		t.Errorf("重复项目应该被拒绝，期望 %v，得到 %v", ER_Duplicate, result)
	}

	// 验证原始项目未被替换
	if list.Items[0].value != 1 {
		t.Errorf("原始项目不应被替换，期望值 1，得到 %d", list.Items[0].value)
	}
}

// 测试容量限制和项目替换
func TestOrderedList_CapacityAndReplacement(t *testing.T) {
	ctx := context.Background()
	list := NewOrderedList[*MockItem](3)

	// 添加三个初始项目
	items := []*MockItem{
		NewMockItem(1, 1.0, "1"),
		NewMockItem(2, 1.0, "2"),
		NewMockItem(3, 1.0, "3"),
	}

	for _, item := range items {
		list.Enqueue(ctx, item)
	}

	// 尝试添加更好的项目
	betterItem := NewMockItem(4, 1.0, "4")
	result := list.Enqueue(ctx, betterItem)
	if result != ER_Enqueued {
		t.Errorf("更好的项目应该被接受，期望 %v，得到 %v", ER_Enqueued, result)
	}

	// 验证最差的项目被移除
	if !items[0].dropped {
		t.Error("最差的项目应该被移除")
	}
	if items[0].reason != ER_LowGain {
		t.Errorf("移除原因错误，期望 %v，得到 %v", ER_LowGain, items[0].reason)
	}

	// 尝试添加更差的项目
	worseItem := NewMockItem(0, 1.0, "0")
	result = list.Enqueue(ctx, worseItem)
	if result != ER_LowGain {
		t.Errorf("更差的项目应该被拒绝，期望 %v，得到 %v", ER_LowGain, result)
	}
}

// 测试边界情况
func TestOrderedList_EdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("空列表", func(t *testing.T) {
		list := NewOrderedList[*MockItem](3)
		if len(list.Items) != 0 {
			t.Errorf("新列表应该为空，但长度为 %d", len(list.Items))
		}

		// 测试空列表的 Peak 和 Pop 操作
		defer func() {
			if r := recover(); r == nil {
				t.Error("空列表调用 Peak 应该触发 panic")
			}
		}()
		list.Peak()
	})

	t.Run("空列表Pop", func(t *testing.T) {
		list := NewOrderedList[*MockItem](3)
		defer func() {
			if r := recover(); r == nil {
				t.Error("空列表调用 Pop 应该触发 panic")
			}
		}()
		list.Pop()
	})

	t.Run("容量为1的列表", func(t *testing.T) {
		list := NewOrderedList[*MockItem](1)

		// 添加第一个项目
		item1 := NewMockItem(1, 1.0, "1")
		result := list.Enqueue(ctx, item1)
		if result != ER_Enqueued {
			t.Errorf("第一个项目入队失败，期望 %v，得到 %v", ER_Enqueued, result)
		}

		// 添加更好的项目
		item2 := NewMockItem(2, 1.0, "2")
		result = list.Enqueue(ctx, item2)
		if result != ER_Enqueued {
			t.Errorf("更好的项目应该被接受，期望 %v，得到 %v", ER_Enqueued, result)
		}

		// 验证只保留了更好的项目
		if len(list.Items) != 1 || list.Items[0].value != 2 {
			t.Errorf("列表应该只包含值为 2 的项目，但得到 %v", list.Items[0])
		}
	})

	t.Run("相同主值不同次值", func(t *testing.T) {
		list := NewOrderedList[*MockItem](3)

		items := []*MockItem{
			NewMockItem(1, 1.0, "1"),
			NewMockItem(1, 2.0, "2"),
			NewMockItem(1, 3.0, "3"),
		}

		for _, item := range items {
			list.Enqueue(ctx, item)
		}

		// 验证按次值排序
		for i := 0; i < len(items); i++ {
			expectedSubValue := 3.0 - float64(i)
			if list.Items[i].subValue != expectedSubValue {
				t.Errorf("位置 %d 的项目次值错误，期望 %.1f，得到 %.1f",
					i, expectedSubValue, list.Items[i].subValue)
			}
		}
	})
}

// 测试 Peak 和 Pop 操作
func TestOrderedList_PeakAndPop(t *testing.T) {
	ctx := context.Background()
	list := NewOrderedList[*MockItem](3)

	// 添加项目
	items := []*MockItem{
		NewMockItem(1, 1.0, "1"),
		NewMockItem(2, 1.0, "2"),
		NewMockItem(3, 1.0, "3"),
	}

	for _, item := range items {
		list.Enqueue(ctx, item)
	}

	// 测试 Peak
	peak := list.Peak()
	if peak.value != 3 {
		t.Errorf("Peak 返回错误，期望值 3，得到 %d", peak.value)
	}
	if len(list.Items) != 3 {
		t.Errorf("Peak 不应改变列表长度，期望 3，得到 %d", len(list.Items))
	}

	// 测试 Pop
	popped := list.Pop()
	if popped.value != 3 {
		t.Errorf("Pop 返回错误，期望值 3，得到 %d", popped.value)
	}
	if len(list.Items) != 2 {
		t.Errorf("Pop 后列表长度错误，期望 2，得到 %d", len(list.Items))
	}
	if _, exists := list.signatures[popped.GetSignature()]; exists {
		t.Error("Pop 后签名应该被移除")
	}
}

// 测试签名管理
func TestOrderedList_SignatureManagement(t *testing.T) {
	ctx := context.Background()
	list := NewOrderedList[*MockItem](3)

	// 添加项目
	item := NewMockItem(1, 1.0, "test")
	list.Enqueue(ctx, item)

	// 验证签名已添加
	if _, exists := list.signatures[item.GetSignature()]; !exists {
		t.Error("签名应该被添加到映射中")
	}

	// Pop 项目
	list.Pop()

	// 验证签名已移除
	if _, exists := list.signatures[item.GetSignature()]; exists {
		t.Error("签名应该从映射中移除")
	}
}

// 测试大量操作的正确性
func TestOrderedList_BulkOperations(t *testing.T) {
	ctx := context.Background()
	list := NewOrderedList[*MockItem](5)

	// 添加 10 个项目
	for i := 0; i < 10; i++ {
		item := NewMockItem(i, float64(i), fmt.Sprintf("%d", i))
		list.Enqueue(ctx, item)
	}

	// 验证只保留了最好的 5 个项目
	if len(list.Items) != 5 {
		t.Errorf("列表长度错误，期望 5，得到 %d", len(list.Items))
	}

	// 验证顺序
	for i := 0; i < 5; i++ {
		expectedValue := 9 - i
		if list.Items[i].value != expectedValue {
			t.Errorf("位置 %d 的项目值错误，期望 %d，得到 %d",
				i, expectedValue, list.Items[i].value)
		}
	}

	// 验证签名数量
	if len(list.signatures) != 5 {
		t.Errorf("签名映射大小错误，期望 5，得到 %d", len(list.signatures))
	}
}

// 测试插入位置
func TestOrderedList_InsertPosition(t *testing.T) {
	ctx := context.Background()
	list := NewOrderedList[*MockItem](5)

	// 测试插入到最前面
	t.Run("插入到最前面", func(t *testing.T) {
		list.Enqueue(ctx, NewMockItem(1, 1.0, "1"))
		list.Enqueue(ctx, NewMockItem(2, 1.0, "2"))
		best := NewMockItem(3, 1.0, "3")
		list.Enqueue(ctx, best)
		if list.Items[0] != best {
			t.Error("最好的项目应该在最前面")
		}
	})

	// 清空列表
	list = NewOrderedList[*MockItem](5)

	// 测试插入到中间
	t.Run("插入到中间", func(t *testing.T) {
		list.Enqueue(ctx, NewMockItem(3, 1.0, "3"))
		list.Enqueue(ctx, NewMockItem(1, 1.0, "1"))
		middle := NewMockItem(2, 1.0, "2")
		list.Enqueue(ctx, middle)
		if list.Items[1] != middle {
			t.Error("项目应该插入到中间位置")
		}
	})

	// 清空列表
	list = NewOrderedList[*MockItem](5)

	// 测试插入到最后
	t.Run("插入到最后", func(t *testing.T) {
		list.Enqueue(ctx, NewMockItem(3, 1.0, "3"))
		list.Enqueue(ctx, NewMockItem(2, 1.0, "2"))
		worst := NewMockItem(1, 1.0, "1")
		list.Enqueue(ctx, worst)
		if list.Items[2] != worst {
			t.Error("最差的项目应该在最后面")
		}
	})
}

// 测试 Unit 类型
func TestUnit(t *testing.T) {
	var u Unit
	var u2 Unit

	// 验证 Unit 类型的零值可以直接使用
	list := NewOrderedList[*MockItem](3)
	list.signatures["test"] = u

	// 验证可以作为 map 值使用
	m := make(map[string]Unit)
	m["test"] = u2

	// 验证可以比较相等
	if u != u2 {
		t.Error("Unit 类型的零值应该相等")
	}
}
