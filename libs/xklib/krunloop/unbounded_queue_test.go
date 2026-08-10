package krunloop

import (
	"context"
	"testing"
	"time"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
)

// TestResource implements CriticalResource for testing
type TestResource struct{}

func (tr *TestResource) IsResource() {}

// TestEvent is a test event type
type TestEvent struct {
	createTimeMs int64 // time when the event was created
	value        int
	execCh       chan int // For verification
}

func (e *TestEvent) GetCreateTimeMs() int64 {
	return e.createTimeMs
}
func (e *TestEvent) GetName() string {
	return "TestEvent"
}

func (e *TestEvent) Process(ctx context.Context, _ *TestResource) {
	select {
	case e.execCh <- e.value:
	default:
	}
}

func newTestEvent(value int) *TestEvent {
	return &TestEvent{
		createTimeMs: kcommon.GetWallTimeMs(),
		value:        value,
		execCh:       make(chan int, 1),
	}
}

// Test basic operations
func TestUnboundedQueueBasicOperations(t *testing.T) {
	ctx := context.Background()
	q := NewUnboundedQueue[*TestResource](ctx)

	// Test enqueue and dequeue
	q.Enqueue(newTestEvent(1))
	q.Enqueue(newTestEvent(2))
	q.Enqueue(newTestEvent(3))

	// Verify queue size
	if size := q.GetSize(); size != 3 {
		t.Errorf("Queue size = %d, expected 3", size)
	}

	// Verify dequeue order
	for i := 1; i <= 3; i++ {
		item, ok := <-q.GetOutputChan()
		if !ok {
			t.Errorf("Dequeue returned ok = false, expected true")
			continue
		}
		if e, ok := item.(*TestEvent); !ok || e.value != i {
			t.Errorf("Dequeue returned value %v, expected %d", e.value, i)
		}
	}

	// Verify queue is empty
	if size := q.GetSize(); size != 0 {
		t.Errorf("Queue size = %d, expected 0", size)
	}
}

// Test close operation
func TestUnboundedQueueClose(t *testing.T) {
	ctx := context.Background()
	queryCtx, cancel := context.WithCancel(ctx)

	q := NewUnboundedQueue[*TestResource](queryCtx)

	// Enqueue some data
	for i := 1; i <= 3; i++ {
		q.Enqueue(newTestEvent(i))
	}

	// Close the queue
	cancel()
	time.Sleep(1 * time.Millisecond)

	// Verify post-close enqueue is a LOUD DROP: returns without blocking,
	// size unchanged, dropped metric incremented.
	// （曾断言 panic；被真实代码推翻——停机掉队定时器是合法投递者，见 Enqueue 注释）
	droppedBefore, _ := RunLoopEnqueueDroppedMetric.GetTimeSequence(ctx, "TestEvent").Get()
	sizeBefore := q.GetSize()
	q.Enqueue(newTestEvent(4))
	if size := q.GetSize(); size != sizeBefore {
		t.Errorf("post-close enqueue must not grow queue: before=%d after=%d", sizeBefore, size)
	}
	droppedAfter, _ := RunLoopEnqueueDroppedMetric.GetTimeSequence(ctx, "TestEvent").Get()
	if droppedAfter != droppedBefore+1 {
		t.Errorf("dropped metric must increment: before=%d after=%d", droppedBefore, droppedAfter)
	}

	// Verify can read all data
	count := 0
	for {
		_, ok := <-q.GetOutputChan()
		if !ok {
			break
		}
		count++
	}

	if count != 0 {
		t.Errorf("Read %d items after closing, expected 0", count)
	}
}

// Test context cancellation
func TestUnboundedQueueContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	q := NewUnboundedQueue[*TestResource](ctx)

	// Enqueue some data
	for i := 1; i <= 3; i++ {
		q.Enqueue(newTestEvent(i))
	}

	// Cancel the context
	cancel()

	// Wait to ensure processing is complete
	time.Sleep(100 * time.Millisecond)

	// Verify post-cancel enqueue is a loud drop (no block, no growth)
	sizeBefore := q.GetSize()
	q.Enqueue(newTestEvent(4))
	if size := q.GetSize(); size != sizeBefore {
		t.Errorf("post-cancel enqueue must not grow queue: before=%d after=%d", sizeBefore, size)
	}

	// Verify Dequeue returns false
	_, ok := <-q.GetOutputChan()
	if ok {
		t.Error("Dequeue returned ok = true after context cancellation")
	}
}
