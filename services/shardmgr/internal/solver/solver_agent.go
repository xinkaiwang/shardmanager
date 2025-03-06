package solver

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
)

// SolverGroup manages multiple SolverDrivers
type SolverGroup struct {
	ThreadPool    *ThreadPool
	Snapshot      *costfunc.Snapshot
	SolverDrivers map[SolverType]*SolverDriver
}

func NewSolverGroup(ctx context.Context) *SolverGroup {
	return &SolverGroup{
		ThreadPool: NewThreadPool(ctx, 2, "SolverGroup"),
	}
}

func (sa *SolverGroup) AddSolver(ctx context.Context, solverName SolverType, solver Solver) {
	driver := NewSolverDriver(ctx, solverName, sa, solver, sa.ThreadPool.EnqueueTask)
	sa.SolverDrivers[solverName] = driver
}

// SolverDriver manages threads for 1 solver
type SolverDriver struct {
	ctx         context.Context
	name        SolverType
	parent      *SolverGroup
	solver      Solver
	enqueueTask func(task Task)
	threads     []*DriverThread
}

func NewSolverDriver(ctx context.Context, name SolverType, parent *SolverGroup, solver Solver, enqueue func(task Task)) *SolverDriver {
	driver := &SolverDriver{
		name:        name,
		parent:      parent,
		solver:      solver,
		enqueueTask: enqueue,
	}
	driver.threadCountWatcher()
	return driver
}

func (sd *SolverDriver) threadCountWatcher() {
	expectedThreadCount := sd.expectedThreadCount()
	for len(sd.threads) < expectedThreadCount {
		sd.threads = append(sd.threads, NewDriverThread(sd, 10*1000))
	}
	for len(sd.threads) > expectedThreadCount {
		sd.threads[len(sd.threads)-1].Stop()
		sd.threads = sd.threads[:len(sd.threads)-1]
	}
	kcommon.ScheduleRun(60*1000, func() {
		sd.threadCountWatcher()
	})
}

func (sd *SolverDriver) expectedThreadCount() int {
	cfg := GetCurrentSolverConfigProvider().GetByName(sd.name)
	if cfg == nil {
		return 0
	}
	if !cfg.SolverEnabled {
		return 0
	}
	qpm := cfg.RunPerMinute
	if qpm <= 0 {
		return 0
	}
	return int(float64(qpm)/6.0 + 0.5) // each thread runs every 10 seconds, so 6 threads per minute
}

type DriverThread struct {
	parent         *SolverDriver
	sleepPerLoopMs int
	stop           bool
}

func NewDriverThread(parent *SolverDriver, sleepPerLoopMs int) *DriverThread {
	dt := &DriverThread{
		parent:         parent,
		sleepPerLoopMs: sleepPerLoopMs,
		stop:           false,
	}
	go dt.run()
	return dt
}

func (dt *DriverThread) Stop() {
	dt.stop = true
}

func (dt *DriverThread) run() {
	for !dt.stop {
		task := NewDriverThreadTask(dt, dt.parent.name)
		dt.parent.enqueueTask(task)
		<-task.done

		// sleep
		ch := make(chan struct{})
		kcommon.ScheduleRun(dt.sleepPerLoopMs, func() {
			close(ch)
		})
		<-ch
	}
}

// DriverThreadTask implements Task interface
type DriverThreadTask struct {
	name   SolverType
	parent *DriverThread
	done   chan struct{}
}

func NewDriverThreadTask(parent *DriverThread, name SolverType) *DriverThreadTask {
	return &DriverThreadTask{
		parent: parent,
		name:   name,
		done:   make(chan struct{}),
	}
}

func (dtt *DriverThreadTask) GetName() string {
	return string(dtt.name)
}

func (dtt *DriverThreadTask) Execute() {
	dtt.parent.parent.solver.FindProposal(dtt.parent.parent.ctx, dtt.parent.parent.parent.Snapshot)
	close(dtt.done)
}
