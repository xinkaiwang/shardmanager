package solver

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kmetrics"
)

var (
	ThreadPoolElapsedMsMetrics = kmetrics.CreateKmetric(context.Background(), "thread_pool_elapsed_ms", "desc", []string{"name", "event"})
)

type Task interface {
	GetName() string
	Execute()
}

type ThreadPool struct {
	name         string
	agentThreads []*AgentThread
	ch           chan Task
}

// NewThreadPool: name is used for logging/metrics purposes only
func NewThreadPool(ctx context.Context, threadNum int, name string) *ThreadPool {
	tp := &ThreadPool{
		name: name,
		ch:   make(chan Task, 1000),
	}
	tp.agentThreads = make([]*AgentThread, threadNum)
	for i := 0; i < threadNum; i++ {
		tp.agentThreads[i] = NewAgentThread(ctx, tp)
	}
	return tp
}

func (tp *ThreadPool) EnqueueTask(task Task) {
	tp.ch <- task
}

func (tp *ThreadPool) Stop() {
	for _, td := range tp.agentThreads {
		td.Stop()
	}
}
func (tp *ThreadPool) WaitForExit() {
	for _, td := range tp.agentThreads {
		td.WaitForExit()
	}
}

func (tp *ThreadPool) StopAndWaitForExit() {
	for _, td := range tp.agentThreads {
		td.Stop()
	}
	for _, td := range tp.agentThreads {
		td.WaitForExit()
	}
}

type AgentThread struct {
	parent  *ThreadPool
	cancel  context.CancelFunc
	stopped chan struct{}
}

func NewAgentThread(ctx context.Context, parent *ThreadPool) *AgentThread {
	td := &AgentThread{
		parent:  parent,
		stopped: make(chan struct{}),
	}
	go td.Run(ctx)
	return td
}

func (td *AgentThread) Run(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	td.cancel = cancel
	err := kcommon.TryCatchRun(ctx, func() {
		td.run(ctx)
	})
	if err != nil {
		klogging.Fatal(ctx).With("err", err).Log("AgentThreadRun", "exit with panic")
	}
	close(td.stopped)
}

func (td *AgentThread) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case task := <-td.parent.ch:
			taskName := task.GetName()
			startTime := kcommon.GetMonoTimeMs()
			task.Execute()
			elapsedMs := kcommon.GetMonoTimeMs() - startTime
			ThreadPoolElapsedMsMetrics.GetTimeSequence(ctx, td.parent.name, taskName).Add(elapsedMs)
		}
	}
}

func (td *AgentThread) Stop() {
	if td.cancel != nil {
		td.cancel()
		td.cancel = nil
	}
}

func (td *AgentThread) WaitForExit() {
	// 等待线程停止
	<-td.stopped
}
