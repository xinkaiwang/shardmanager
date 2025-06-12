package solver

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kmetrics"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
)

var (
	solverGroupInvokeMsMetrics          = kmetrics.CreateKmetric(context.Background(), "solver_proposal_find_ms", "solver invoke ms", []string{"solver"})
	solverGroupProposalGeneratedMetrics = kmetrics.CreateKmetric(context.Background(), "solver_proposal_generated", "how many proposal get generated", []string{"solver"}).CountOnly()
	solverGroupProposalEnqueueMetrics   = kmetrics.CreateKmetric(context.Background(), "solver_proposal_enqueue", "how many proposal get enqueue", []string{"solver"}).CountOnly()
)

func metricsInitSolverGroup(ctx context.Context, solverName string) {
	solverGroupInvokeMsMetrics.GetTimeSequence(ctx, solverName).Touch()
	solverGroupProposalGeneratedMetrics.GetTimeSequence(ctx, solverName).Touch()
	solverGroupProposalEnqueueMetrics.GetTimeSequence(ctx, solverName).Touch()
	costfunc.ProposalDropOutMetrics.GetTimeSequence(ctx, solverName, "conflict").Touch()
	costfunc.ProposalDropOutMetrics.GetTimeSequence(ctx, solverName, "low_gain").Touch()
	costfunc.ProposalAcceptedMetrics.GetTimeSequence(ctx, solverName).Touch()
	costfunc.ProposalSuccMetrics.GetTimeSequence(ctx, solverName).Touch()
	costfunc.ProposalFailMetrics.GetTimeSequence(ctx, solverName).Touch()
}

type SnapshotListener interface {
	OnSnapshot(ctx context.Context, snapshot *costfunc.Snapshot, reason string)
	StopAndWaitForExit()
}

// SolverGroup manages multiple SolverDrivers
type SolverGroup struct {
	ThreadPool *ThreadPool
	// Snapshot may have race condition, so we need atomic operations
	Snapshot         atomic.Value // *costfunc.Snapshot
	SolverDrivers    map[SolverType]*SolverDriver
	enqueueProposals func(ctx context.Context, proposal *costfunc.Proposal) common.EnqueueResult
}

func NewSolverGroup(ctx context.Context, snapshot *costfunc.Snapshot, enqueueProposals func(ctx context.Context, proposal *costfunc.Proposal) common.EnqueueResult) *SolverGroup {
	group := &SolverGroup{
		ThreadPool:       NewThreadPool(ctx, 2 /* how many CPU cores */, "SolverGroup"),
		SolverDrivers:    map[SolverType]*SolverDriver{},
		enqueueProposals: enqueueProposals,
	}
	group.storeSnapshot(snapshot)
	return group
}

func (sa *SolverGroup) AddSolver(ctx context.Context, solver Solver) {
	solverName := solver.GetType()
	metricsInitSolverGroup(ctx, string(solverName))
	driver := NewSolverDriver(ctx, solverName, sa, solver, sa.ThreadPool.EnqueueTask)
	sa.SolverDrivers[solverName] = driver
}

func (sa *SolverGroup) OnSnapshot(ctx context.Context, snapshot *costfunc.Snapshot, reason string) {
	klogging.Info(ctx).With("cost", snapshot.GetCost(ctx)).With("snapshotId", snapshot.SnapshotId).With("reason", reason).Log("SolverGroup", "OnSnapshot")
	sa.storeSnapshot(snapshot)
}

func (sa *SolverGroup) storeSnapshot(snapshot *costfunc.Snapshot) {
	sa.Snapshot.Store(snapshot)
}

func (sa *SolverGroup) loadSnapshot() *costfunc.Snapshot {
	return sa.Snapshot.Load().(*costfunc.Snapshot)
}

func (sa *SolverGroup) Stop() {
	for _, driver := range sa.SolverDrivers {
		driver.Stop()
	}
	sa.ThreadPool.Stop()
}

func (sa *SolverGroup) StopAndWaitForExit() {
	for _, driver := range sa.SolverDrivers {
		driver.StopAndWaitForExit()
	}
	// 等待所有线程池中的线程退出
	sa.ThreadPool.StopAndWaitForExit()
}

// SolverDriver manages threads for 1 solver
type SolverDriver struct {
	ctx            context.Context
	name           SolverType
	parent         *SolverGroup
	solver         Solver
	enqueueTask    func(task Task)
	threadsMutex   sync.Mutex // 保护 threads 数组
	threads        []*DriverThread
	sleepPerLoopMs atomic.Int64 // 使用原子操作代替互斥锁保护
	threadIdCt     int          // for logging/debug only, not used in logic
}

func NewSolverDriver(ctx context.Context, name SolverType, parent *SolverGroup, solver Solver, enqueue func(task Task)) *SolverDriver {
	driver := &SolverDriver{
		ctx:         ctx,
		name:        name,
		parent:      parent,
		solver:      solver,
		enqueueTask: enqueue,
	}
	// 设置初始值为1000
	driver.sleepPerLoopMs.Store(1000)
	driver.threadCountWatcher(ctx)
	return driver
}

func (sd *SolverDriver) threadCountWatcher(ctx context.Context) {
	expectedThreadCount, newSleepPerLoopMs := sd.expectedThreadCount()
	// 使用原子操作更新 sleepPerLoopMs
	sd.sleepPerLoopMs.Store(int64(newSleepPerLoopMs))

	// 使用互斥锁保护对 threads 数组的访问
	sd.threadsMutex.Lock()
	defer sd.threadsMutex.Unlock()

	// 添加需要的线程
	for len(sd.threads) < expectedThreadCount {
		sd.threads = append(sd.threads, NewDriverThread(ctx, sd, "sv_"+string(sd.name)+"_"+strconv.Itoa(sd.threadIdCt)))
		sd.threadIdCt++
	}

	// 移除多余的线程
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

func (sd *SolverDriver) Stop() {
	// 使用互斥锁保护对 threads 数组的访问
	sd.threadsMutex.Lock()
	defer sd.threadsMutex.Unlock()

	for _, thread := range sd.threads {
		thread.Stop()
	}
}

func (sd *SolverDriver) StopAndWaitForExit() {
	// 使用互斥锁保护对 threads 数组的访问
	sd.threadsMutex.Lock()
	defer sd.threadsMutex.Unlock()

	for _, thread := range sd.threads {
		thread.Stop()
	}
	for _, thread := range sd.threads {
		thread.WaitForExit()
	}
}

type DriverThread struct {
	ctx        context.Context
	parent     *SolverDriver
	stop       chan struct{} // closed means should stop
	threadName string        // for logging/debug only
	stopped    chan struct{} // 用于通知线程停止
}

func NewDriverThread(ctx context.Context, parent *SolverDriver, threadName string) *DriverThread {
	ctx2, info := klogging.GetOrCreateCtxInfo(ctx)
	info.With("traceId", threadName)
	dt := &DriverThread{
		ctx:        ctx2,
		parent:     parent,
		threadName: threadName,
		stop:       make(chan struct{}), // 用于控制线程停止
		stopped:    make(chan struct{}),
	}
	go dt.run()
	return dt
}

func (dt *DriverThread) Stop() {
	close(dt.stop) // 关闭 stop 通道，通知线程停止
}

func (dt *DriverThread) run() {
	// 使用原子操作读取 sleepPerLoopMs
	sleepMs := int(dt.parent.sleepPerLoopMs.Load())
	// initial sleep
	initalSleepMs := kcommon.RandomInt(dt.ctx, sleepMs)
	stop := false
	if initalSleepMs > 0 {
		ch := make(chan struct{})
		kcommon.ScheduleRun(initalSleepMs, func() {
			close(ch)
		})
		select {
		case <-ch:
			// 正常唤醒，继续下一轮
		case <-dt.stop:
			stop = true
		}
	}

	for !stop {
		task := NewDriverThreadTask(dt.ctx, dt, dt.parent.name)
		dt.parent.enqueueTask(task)
		<-task.done

		// sleep - 使用原子操作读取最新的 sleepPerLoopMs
		sleepMs = int(dt.parent.sleepPerLoopMs.Load())
		ch := make(chan struct{})
		kcommon.ScheduleRun(sleepMs, func() {
			close(ch)
		})
		select {
		case <-ch:
			// 正常唤醒，继续下一轮
		case <-dt.stop:
			stop = true
		}
	}
	klogging.Info(dt.ctx).With("threadName", dt.threadName).Log("DriverThread", "thread stopped")
	close(dt.stopped)
}

func (dt *DriverThread) StopAndWaitForExit() {
	dt.Stop()
	dt.WaitForExit()
}

func (dt *DriverThread) WaitForExit() {
	// 等待线程停止
	<-dt.stopped
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
	startTime := kcommon.GetWallTimeMs()
	proposal := dtt.parent.parent.solver.FindProposal(dtt.parent.parent.ctx, dtt.parent.parent.parent.loadSnapshot())
	elapsedMs := kcommon.GetWallTimeMs() - startTime
	klogging.Verbose(dtt.ctx).With("solver", dtt.name).With("elapsedMs", elapsedMs).Log("SolverGroup", "Execute")
	solverGroupInvokeMsMetrics.GetTimeSequence(dtt.ctx, string(dtt.name)).Add(elapsedMs)
	if proposal == nil {
		close(dtt.done)
		return
	}
	solverGroupProposalGeneratedMetrics.GetTimeSequence(dtt.ctx, string(dtt.name)).Add(1)
	// klogging.Verbose(dtt.ctx).With("solver", dtt.name).With("proposalId", proposal.ProposalId).With("signature", proposal.Signature).With("gain", proposal.Gain).With("base", proposal.BasedOn).Log("SolverGroup", "NewProposal")
	result := dtt.parent.parent.parent.enqueueProposals(dtt.ctx, proposal)
	if result == common.ER_Enqueued {
		solverGroupProposalEnqueueMetrics.GetTimeSequence(dtt.ctx, string(dtt.name)).Add(1)
	}
	close(dtt.done)
}
