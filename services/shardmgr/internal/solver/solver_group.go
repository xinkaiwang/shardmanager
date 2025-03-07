package solver

import (
	"context"
	"sync/atomic"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
)

// SolverGroup manages multiple SolverDrivers
type SolverGroup struct {
	ThreadPool *ThreadPool
	// Snapshot may have race condition, so we need atomic operations
	Snapshot         atomic.Value // *costfunc.Snapshot
	SolverDrivers    map[SolverType]*SolverDriver
	enqueueProposals func(proposal *costfunc.Proposal) common.EnqueueResult
}

func NewSolverGroup(ctx context.Context, snapshot *costfunc.Snapshot, enqueueProposals func(proposal *costfunc.Proposal) common.EnqueueResult) *SolverGroup {
	group := &SolverGroup{
		ThreadPool:       NewThreadPool(ctx, 2 /* how many CPU cores */, "SolverGroup"),
		SolverDrivers:    map[SolverType]*SolverDriver{},
		enqueueProposals: enqueueProposals,
	}
	group.storeSnapshot(snapshot)
	// group.AddSolver(ctx, NewSoftSolver())
	// group.AddSolver(ctx, NewAssignSolver())
	// group.AddSolver(ctx, NewUnassignSolver())
	return group
}

func (sa *SolverGroup) AddSolver(ctx context.Context, solver Solver) {
	solverName := solver.GetType()
	driver := NewSolverDriver(ctx, solverName, sa, solver, sa.ThreadPool.EnqueueTask)
	sa.SolverDrivers[solverName] = driver
}

func (sa *SolverGroup) SetCurrentSnapshot(snapshot *costfunc.Snapshot) {
	sa.storeSnapshot(snapshot)
}

func (sa *SolverGroup) storeSnapshot(snapshot *costfunc.Snapshot) {
	sa.Snapshot.Store(snapshot)
}

func (sa *SolverGroup) loadSnapshot() *costfunc.Snapshot {
	return sa.Snapshot.Load().(*costfunc.Snapshot)
}

// SolverDriver manages threads for 1 solver
type SolverDriver struct {
	ctx            context.Context
	name           SolverType
	parent         *SolverGroup
	solver         Solver
	enqueueTask    func(task Task)
	threads        []*DriverThread
	sleepPerLoopMs int
}

func NewSolverDriver(ctx context.Context, name SolverType, parent *SolverGroup, solver Solver, enqueue func(task Task)) *SolverDriver {
	driver := &SolverDriver{
		ctx:            ctx,
		name:           name,
		parent:         parent,
		solver:         solver,
		enqueueTask:    enqueue,
		sleepPerLoopMs: 1000,
	}
	driver.threadCountWatcher(ctx)
	return driver
}

func (sd *SolverDriver) threadCountWatcher(ctx context.Context) {
	expectedThreadCount, sleepPerLoopMs := sd.expectedThreadCount()
	// klogging.Info(ctx).With("solver", sd.name).With("expectedThreadCount", expectedThreadCount).With("sleepPerLoopMs", sleepPerLoopMs).Log("SolverDriver", "threadCountWatcher")
	sd.sleepPerLoopMs = sleepPerLoopMs
	for len(sd.threads) < expectedThreadCount {
		sd.threads = append(sd.threads, NewDriverThread(ctx, sd))
	}
	for len(sd.threads) > expectedThreadCount {
		sd.threads[len(sd.threads)-1].Stop()
		sd.threads = sd.threads[:len(sd.threads)-1]
	}
	kcommon.ScheduleRun(1*1000, func() {
		sd.threadCountWatcher(ctx)
	})
}

func (sd *SolverDriver) expectedThreadCount() (threadCount int, sleepPerLoopMs int) {
	// what is the target QPM?
	cfg := GetCurrentSolverConfigProvider().GetByName(sd.name)
	if cfg == nil {
		return 0, 1000
	}
	if !cfg.SolverEnabled {
		return 0, 1000
	}
	qpm := cfg.RunPerMinute
	if qpm <= 0 {
		return 0, 1000
	}
	// given this QPM, how many threads (and sleepPerLoopMs) do we need?
	if qpm >= 60 {
		threadCount = qpm / 60
		sleepPerLoopMs = 1000
		return
	} else {
		threadCount = 1
		sleepPerLoopMs = int(60000.0 / (float64(qpm) / float64(threadCount)))
		return
	}
}

type DriverThread struct {
	ctx    context.Context
	parent *SolverDriver
	stop   bool
}

func NewDriverThread(ctx context.Context, parent *SolverDriver) *DriverThread {
	dt := &DriverThread{
		ctx:    ctx,
		parent: parent,
		stop:   false,
	}
	go dt.run()
	return dt
}

func (dt *DriverThread) Stop() {
	dt.stop = true
}

func (dt *DriverThread) run() {
	// initial sleep
	initalSleepMs := kcommon.RandomInt(dt.ctx, dt.parent.sleepPerLoopMs)
	if initalSleepMs > 0 {
		ch := make(chan struct{})
		kcommon.ScheduleRun(initalSleepMs, func() {
			close(ch)
		})
		<-ch
	}
	for !dt.stop {
		task := NewDriverThreadTask(dt.ctx, dt, dt.parent.name)
		dt.parent.enqueueTask(task)
		<-task.done

		// sleep
		ch := make(chan struct{})
		kcommon.ScheduleRun(dt.parent.sleepPerLoopMs, func() {
			close(ch)
		})
		<-ch
	}
}

// DriverThreadTask implements Task interface
type DriverThreadTask struct {
	ctx    context.Context
	name   SolverType
	parent *DriverThread
	done   chan struct{}
}

func NewDriverThreadTask(ctx context.Context, parent *DriverThread, name SolverType) *DriverThreadTask {
	return &DriverThreadTask{
		ctx:    ctx,
		parent: parent,
		name:   name,
		done:   make(chan struct{}),
	}
}

func (dtt *DriverThreadTask) GetName() string {
	return string(dtt.name)
}

func (dtt *DriverThreadTask) Execute() {
	proposal := dtt.parent.parent.solver.FindProposal(dtt.parent.parent.ctx, dtt.parent.parent.parent.loadSnapshot())
	result := dtt.parent.parent.parent.enqueueProposals(proposal)
	if result != common.ER_Enqueued {
		klogging.Debug(dtt.ctx).With("solver", dtt.name).With("result", result).Log("SolverGroup", "EnqueueProposal")
	}
	close(dtt.done)
}
