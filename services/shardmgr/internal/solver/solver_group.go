package solver

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
)

// SolverGroup manages multiple SolverDrivers
type SolverGroup struct {
	ThreadPool       *ThreadPool
	Snapshot         *costfunc.Snapshot
	SolverDrivers    map[SolverType]*SolverDriver
	enqueueProposals func(proposal *costfunc.Proposal) common.EnqueueResult
}

func NewSolverGroup(ctx context.Context, snapshot *costfunc.Snapshot, enqueueProposals func(proposal *costfunc.Proposal) common.EnqueueResult) *SolverGroup {
	group := &SolverGroup{
		ThreadPool:       NewThreadPool(ctx, 2 /* how many CPU cores */, "SolverGroup"),
		Snapshot:         snapshot,
		SolverDrivers:    map[SolverType]*SolverDriver{},
		enqueueProposals: enqueueProposals,
	}
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
		ctx:         ctx,
		name:        name,
		parent:      parent,
		solver:      solver,
		enqueueTask: enqueue,
	}
	driver.threadCountWatcher(ctx)
	return driver
}

func (sd *SolverDriver) threadCountWatcher(ctx context.Context) {
	expectedThreadCount := sd.expectedThreadCount()
	for len(sd.threads) < expectedThreadCount {
		sd.threads = append(sd.threads, NewDriverThread(ctx, sd, 1*1000))
	}
	for len(sd.threads) > expectedThreadCount {
		sd.threads[len(sd.threads)-1].Stop()
		sd.threads = sd.threads[:len(sd.threads)-1]
	}
	kcommon.ScheduleRun(1*1000, func() {
		sd.threadCountWatcher(ctx)
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
	return int(float64(qpm)/60.0 + 0.5) // each thread runs every 1 seconds, so 1 threads = 60 QPM
}

type DriverThread struct {
	ctx            context.Context
	parent         *SolverDriver
	sleepPerLoopMs int
	stop           bool
}

func NewDriverThread(ctx context.Context, parent *SolverDriver, sleepPerLoopMs int) *DriverThread {
	dt := &DriverThread{
		ctx:            ctx,
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
		task := NewDriverThreadTask(dt.ctx, dt, dt.parent.name)
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
	proposal := dtt.parent.parent.solver.FindProposal(dtt.parent.parent.ctx, dtt.parent.parent.parent.Snapshot)
	result := dtt.parent.parent.parent.enqueueProposals(proposal)
	if result != common.ER_Enqueued {
		klogging.Debug(dtt.ctx).With("solver", dtt.name).With("result", result).Log("SolverGroup", "EnqueueProposal")
	}
	close(dtt.done)
}
