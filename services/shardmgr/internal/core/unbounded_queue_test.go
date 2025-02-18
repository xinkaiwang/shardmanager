package core

import (
	"context"
	"testing"
	"time"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
)

// 测试用的事件类型
type testEvent struct {
	value  int
	execCh chan int // 用于验证执行
}

func (e *testEvent) GetName() string {
	return "testEvent"
}

func (e *testEvent) Execute(ctx context.Context, _ *ServiceState) {
	select {
	case e.execCh <- e.value:
	default:
	}
}

func newTestEvent(value int) *testEvent {
	return &testEvent{
		value:  value,
		execCh: make(chan int, 1),
	}
}

// 测试基本操作
func TestUnboundedQueueBasicOperations(t *testing.T) {
	ctx := context.Background()
	q := NewUnboundedQueue(ctx)

	// 测试入队和出队
	q.Enqueue(newTestEvent(1))
	q.Enqueue(newTestEvent(2))
	q.Enqueue(newTestEvent(3))

	// 验证队列大小
	if size := q.GetSize(); size != 3 {
		t.Errorf("队列大小 = %d, 期望 3", size)
	}

	// 验证出队顺序
	for i := 1; i <= 3; i++ {
		item, ok := <-q.GetOutputChan()
		if !ok {
			t.Errorf("Dequeue 返回 ok = false, 期望 true")
			continue
		}
		if e, ok := item.(*testEvent); !ok || e.value != i {
			t.Errorf("Dequeue 返回值 %v, 期望 %d", e.value, i)
		}
	}

	// 验证队列为空
	if size := q.GetSize(); size != 0 {
		t.Errorf("队列大小 = %d, 期望 0", size)
	}
}

// 测试关闭操作
func TestUnboundedQueueClose(t *testing.T) {
	ctx := context.Background()
	queryCtx, cancel := context.WithCancel(ctx)

	q := NewUnboundedQueue(queryCtx)

	// 入队一些数据
	for i := 1; i <= 3; i++ {
		q.Enqueue(newTestEvent(i))
	}

	// 关闭队列
	cancel()
	time.Sleep(1 * time.Millisecond)

	// 验证无法继续入队
	ke := kcommon.TryCatchRun(ctx, func() {
		q.Enqueue(newTestEvent(4))
	})
	if ke == nil {
		t.Error("关闭后仍然可以入队")
	}

	// 验证可以读取所有数据
	count := 0
	for {
		_, ok := <-q.GetOutputChan()
		if !ok {
			break
		}
		count++
	}

	if count != 0 {
		t.Errorf("关闭后读取到 %d 个项目, 期望 0 个", count)
	}
}

// 测试上下文取消
func TestUnboundedQueueContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	q := NewUnboundedQueue(ctx)

	// 入队一些数据
	for i := 1; i <= 3; i++ {
		q.Enqueue(newTestEvent(i))
	}

	// 取消上下文
	cancel()

	// 等待一段时间确保处理完成
	time.Sleep(100 * time.Millisecond)

	// 验证无法继续入队
	ke := kcommon.TryCatchRun(ctx, func() {
		q.Enqueue(newTestEvent(4))
	})
	if ke == nil {
		t.Error("关闭后仍然可以入队")
	}

	// 验证 Dequeue 返回 false
	_, ok := <-q.GetOutputChan()
	if ok {
		t.Error("上下文取消后 Dequeue 返回 ok = true")
	}
}
