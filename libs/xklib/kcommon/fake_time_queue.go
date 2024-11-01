package kcommon

type FakeTimerTask struct {
	MonoTimeMs int64
	TaskFunc   func()
}

type TaskQueue struct {
	queue []*FakeTimerTask
}

func (tq *TaskQueue) Len() int {
	return len(tq.queue)
}

func (tq *TaskQueue) Less(i, j int) bool {
	return tq.queue[i].MonoTimeMs < tq.queue[j].MonoTimeMs
}

func (tq *TaskQueue) Swap(i, j int) {
	tq.queue[i], tq.queue[j] = tq.queue[j], tq.queue[i]
}

func (tq *TaskQueue) Push(x interface{}) {
	tq.queue = append(tq.queue, x.(*FakeTimerTask))
}

func (tq *TaskQueue) Pop() interface{} {
	old := tq.queue

	// item to return
	x := old[len(old)-1]
	old[len(old)-1] = nil

	// slice
	tq.queue = old[0 : len(old)-1]
	return x
}

func (tq *TaskQueue) Peek() *FakeTimerTask {
	return tq.queue[0]
}
