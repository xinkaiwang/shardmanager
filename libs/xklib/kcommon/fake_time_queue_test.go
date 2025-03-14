package kcommon

import (
	"container/heap"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTaskQueue(t *testing.T) {
	tq := &TaskQueue{}
	assert.Equal(t, 0, tq.Len())
	heap.Init(tq)

	heap.Push(tq, &FakeTimerTask{
		ScheduledForMs: 1,
		TaskFunc:       func() {},
	})
	heap.Push(tq, &FakeTimerTask{
		ScheduledForMs: 100,
		TaskFunc:       func() {},
	})
	heap.Push(tq, &FakeTimerTask{
		ScheduledForMs: 10,
		TaskFunc:       func() {},
	})
	assert.Equal(t, 3, tq.Len())

	assert.Equal(t, int64(1), heap.Pop(tq).(*FakeTimerTask).ScheduledForMs)
	assert.Equal(t, 2, tq.Len())

	assert.Equal(t, int64(10), heap.Pop(tq).(*FakeTimerTask).ScheduledForMs)
	assert.Equal(t, 1, tq.Len())

	assert.Equal(t, int64(100), heap.Pop(tq).(*FakeTimerTask).ScheduledForMs)
	assert.Equal(t, 0, tq.Len())
}
