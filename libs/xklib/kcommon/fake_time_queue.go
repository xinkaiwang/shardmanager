package kcommon

import "container/heap"

type FakeTimerTask struct {
	ScheduledForMs int64
	TaskFunc       func()
}

type TaskQueue struct {
	queue []*FakeTimerTask
}

func NewTaskQueue() *TaskQueue {
	tq := &TaskQueue{
		queue: make([]*FakeTimerTask, 0),
	}
	heap.Init(tq)
	return tq
}

func (tq *TaskQueue) Len() int {
	l := len(tq.queue)
	return l
}

func (tq TaskQueue) Less(i, j int) bool {
	return tq.queue[i].ScheduledForMs < tq.queue[j].ScheduledForMs
}

func (tq TaskQueue) Swap(i, j int) {
	tq.queue[i], tq.queue[j] = tq.queue[j], tq.queue[i]
}

func (tq *TaskQueue) Push(x interface{}) {
	tq.queue = append(tq.queue, x.(*FakeTimerTask))
}

func (tq *TaskQueue) Pop() interface{} {
	x := tq.queue[len(tq.queue)-1]
	tq.queue = tq.queue[:len(tq.queue)-1]
	return x
}

// Peek returns the next task to be executed,
// return nil if queue empty
func (tq *TaskQueue) Peek() *FakeTimerTask {
	if len(tq.queue) == 0 {
		return nil
	}
	return tq.queue[0]
}
