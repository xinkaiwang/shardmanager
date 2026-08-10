package krunloop

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
)

// UnboundedQueue implements an unbounded queue for events of type IEvent[T]
type UnboundedQueue[T CriticalResource] struct {
	input     chan IEvent[T] // Channel for receiving events
	buffer    []IEvent[T]    // Internal buffer
	output    chan IEvent[T] // Channel for sending events
	closed    atomic.Bool    // Whether the queue is closed (set on every run() exit path)
	size      atomic.Int64   // Current number of elements in the queue
	closeOnce sync.Once      // Ensure output channel is closed only once

	stop     chan struct{} // Channel to signal stop processing
	stopOnce sync.Once     // Stop() is called from multiple paths (XS-002: Run's defer + StopAndWaitForExit) — must be idempotent
	stopped  chan struct{} // Channel to notify when thread has stopped
}

// NewUnboundedQueue creates a new unbounded queue for events of type IEvent[T]
func NewUnboundedQueue[T CriticalResource](ctx context.Context) *UnboundedQueue[T] {
	q := &UnboundedQueue[T]{
		input:  make(chan IEvent[T], 1), // buffer 1 使 Enqueue 在常态下不阻塞（run 及时清空 input；消费者忙时并发的第二个 Enqueue 仍会短暂等待——不是"绝不阻塞"的硬保证）
		buffer: make([]IEvent[T], 0),
		output: make(chan IEvent[T]),
		// closed:    atomic.Bool{},
		size:      atomic.Int64{},
		closeOnce: sync.Once{},
		stop:      make(chan struct{}),
		stopped:   make(chan struct{}),
	}
	// q.closed.Store(false)
	q.size.Store(0)
	go q.run(ctx)
	return q
}

// run handles events in the queue
func (q *UnboundedQueue[T]) run(ctx context.Context) {
	defer func() {
		// Mark closed BEFORE closing output: Enqueue checks this flag and
		// panics, restoring the original "loud rejection after close"
		// semantics without the close(q.input) race the original had
		// (closing a channel that live producers send on panics the
		// producer at a random point; a flag check panics deterministically
		// at the call site with a typed kerror instead).
		q.closed.Store(true)
		// Ensure output channel is closed when thread exits
		q.closeOnce.Do(func() {
			close(q.output)
			close(q.stopped)
		})
	}()

	out := q.output
	stop := false
	for !stop {
		// If buffer is empty, out is nil (blocks send)
		var firstItem IEvent[T]
		if len(q.buffer) > 0 {
			firstItem = q.buffer[0]
			out = q.output
		} else {
			out = nil
		}

		select {
		case item, ok := <-q.input:
			if !ok {
				// input channel is closed, mark queue as closed
				// q.closed.Store(true)
				stop = true
				continue
			}
			// Add to buffer
			q.buffer = append(q.buffer, item)

		case out <- firstItem:
			// Successfully sent, remove sent item
			q.buffer = q.buffer[1:]
			q.size.Add(-1)

		case <-ctx.Done():
			// Context canceled, exit immediately
			// q.closed.Store(true)
			stop = true
		case <-q.stop:
			// Stop signal received, exit immediately
			stop = true
		}
	}
}

// Enqueue adds an element to the queue. This call never blocks (while the
// queue is running). Enqueueing after the queue has stopped drops the event
// LOUDLY — Warn log + runloop_enqueue_dropped_ct metric — and returns.
//
// 语义推导（曾短暂实现为 panic，被真实代码推翻）：后停机投递者有两类——
// (a) 生命周期顺序 bug：需要可见（Warn+metric 足够诊断）；
// (b) 合法掉队者：事件处理中调度的延迟定时器（如 AcceptEvent 的重试）与
//     shutdown 赛跑失败——这不是 bug，事件本该在停机时丢弃，panic 会把
//     优雅停机窗口里的每个良性定时器变成崩溃。
// 修复前的行为则是最坏的：第一个事件无声落入死 buffer（size 计数还错了），
// 第二个永久阻塞（调用方 goroutine 泄漏）。
// 注：与 shutdown 竞态的窗口内事件仍可能落入死 buffer（窄窗口，行为同前）；
// closed 标志保证稳态下是"响亮丢弃"而非"无声吞没/死锁"。
func (q *UnboundedQueue[T]) Enqueue(item IEvent[T]) {
	if q.closed.Load() {
		eveName := item.GetName()
		RunLoopEnqueueDroppedMetric.GetTimeSequence(context.Background(), eveName).Add(1)
		slog.WarnContext(context.Background(), "event dropped: enqueue after queue stopped (shutdown straggler or teardown-order bug)",
			slog.String("event", "EnqueueAfterStop"),
			slog.String("droppedEvent", eveName))
		return
	}
	q.input <- item
	q.size.Add(1)
}

// GetOutputChan returns the channel for receiving elements from the queue.
// If the queue is empty, this call will block.
func (q *UnboundedQueue[T]) GetOutputChan() chan IEvent[T] {
	return q.output
}

// GetSize returns the current number of elements in the queue
func (q *UnboundedQueue[T]) GetSize() int64 {
	return q.size.Load()
}

// // Close closes the queue
// func (q *UnboundedQueue[T]) Close() {
// 	q.closed.Store(true)
// }

func (q *UnboundedQueue[T]) Stop() {
	// Stop the processing thread (idempotent — called from both Run's defer
	// and StopAndWaitForExit)
	q.stopOnce.Do(func() { close(q.stop) })
}

func (q *UnboundedQueue[T]) StopAndWaitForExit() {
	// Stop the processing thread and wait for it to exit
	q.Stop()
	<-q.stopped // Wait for the run thread to exit
	slog.InfoContext(context.Background(), "unbounded queue stopped",
		slog.String("event", "UnboundedQueue.Stopped"))
}
