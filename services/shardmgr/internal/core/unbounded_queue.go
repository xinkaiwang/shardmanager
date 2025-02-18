package core

import (
	"context"
	"sync/atomic"
)

// UnboundedQueue 实现了一个无界队列
type UnboundedQueue struct {
	input  chan IEvent   // 用于接收事件的通道
	buffer []IEvent      // 内部缓冲区
	output chan IEvent   // 用于发送事件的通道
	closed atomic.Bool   // 队列是否已关闭
	size   atomic.Int64  // 当前队列中的元素数量
	doneCh chan struct{} // 用于等待 process goroutine 结束
}

// NewUnboundedQueue 创建一个新的无界队列
func NewUnboundedQueue(ctx context.Context) *UnboundedQueue {
	q := &UnboundedQueue{
		input:  make(chan IEvent, 1), // 缓冲为1，确保 Enqueue 不阻塞
		buffer: make([]IEvent, 0),
		output: make(chan IEvent),
		doneCh: make(chan struct{}),
	}
	go q.process(ctx)
	return q
}

// process 处理队列中的事件
func (q *UnboundedQueue) process(ctx context.Context) {
	defer close(q.doneCh) // 通知 Close 方法处理完成

	var out chan IEvent  // nil channel 永远不会被选中
	var firstItem IEvent // 当前要发送的第一个项目

	for {
		// 根据缓冲区状态设置输出通道和第一个项目
		if len(q.buffer) > 0 {
			out = q.output
			firstItem = q.buffer[0]
		} else {
			out = nil
			firstItem = nil
		}

		select {
		case item, ok := <-q.input:
			if !ok {
				// input 通道已关闭，标记队列为关闭状态
				q.closed.Store(true)
				continue
			}
			// 添加到缓冲区
			q.buffer = append(q.buffer, item)

		case out <- firstItem:
			// 成功发送，移除已发送的项目
			q.buffer = q.buffer[1:]
			q.size.Add(-1)

		case <-ctx.Done():
			// 上下文取消时立即退出
			q.closed.Store(true)
			close(q.input)
			close(q.output)
			return
		}
	}
}

// Enqueue 将一个元素添加到队列中。此调用永不阻塞。
func (q *UnboundedQueue) Enqueue(item IEvent) {
	q.input <- item
	q.size.Add(1)
}

// Dequeue 从队列中获取一个元素。如果队列为空，此调用会阻塞。
func (q *UnboundedQueue) Dequeue() (IEvent, bool) {
	item, ok := <-q.output
	return item, ok
}

// GetSize 返回当前队列中的元素数量
func (q *UnboundedQueue) GetSize() int64 {
	return q.size.Load()
}

// Close 关闭队列
func (q *UnboundedQueue) Close() {
	if !q.closed.CompareAndSwap(false, true) {
		return
	}
	close(q.input) // 关闭输入通道
	<-q.doneCh     // 等待处理完成
}
