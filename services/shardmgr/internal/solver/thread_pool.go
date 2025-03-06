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

type AgentThread struct {
	parent *ThreadPool
}

func NewAgentThread(ctx context.Context, parent *ThreadPool) *AgentThread {
	td := &AgentThread{
		parent: parent,
	}
	go td.Run(ctx)
	return td
}

func (td *AgentThread) Run(ctx context.Context) {
	err := kcommon.TryCatchRun(ctx, func() {
		td.run(ctx)
	})
	if err != nil {
		klogging.Fatal(ctx).With("err", err).Log("AgentThreadRun", "exit with panic")
	}
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

type Task interface {
	GetName() string
	Execute()
}
