package krunloop

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
)

// UnboundedQueue implements an unbounded queue for events of type IEvent[T]
type UnboundedQueue[T CriticalResource] struct {
	input  chan IEvent[T] // Channel for receiving events
	buffer []IEvent[T]    // Internal buffer
	output chan IEvent[T] // Channel for sending events
	// closed    atomic.Bool    // Whether the queue is closed
	size      atomic.Int64 // Current number of elements in the queue
	closeOnce sync.Once    // Ensure output channel is closed only once

	stop    chan struct{} // Channel to signal stop processing
	stopped chan struct{} // Channel to notify when thread has stopped
}

// NewUnboundedQueue creates a new unbounded queue for events of type IEvent[T]
func NewUnboundedQueue[T CriticalResource](ctx context.Context) *UnboundedQueue[T] {
	q := &UnboundedQueue[T]{
		input:  make(chan IEvent[T], 1), // Buffer of 1 to ensure Enqueue doesn't block
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

// Enqueue adds an element to the queue. This call never blocks.
func (q *UnboundedQueue[T]) Enqueue(item IEvent[T]) {
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
	// Stop the processing thread
	close(q.stop)
}

func (q *UnboundedQueue[T]) StopAndWaitForExit() {
	// Stop the processing thread and wait for it to exit
	q.Stop()
	<-q.stopped // Wait for the run thread to exit
	klogging.Info(context.Background()).Log("UnboundedQueue", "stopped")
}
