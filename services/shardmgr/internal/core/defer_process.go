package core

import (
	"context"
	"sync"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
)

// DeferProcess: useful when you want to maximize the batching and still keep the max delay under control
// for example, when you set the delay to 100ms, the first event will schedule a run after 100ms, if another event try to shcedule run within 100ms, it will be ignored
type DeferProcess struct {
	parent   *ServiceState
	delayMs  int
	mu       sync.Mutex
	inFlight bool
}

func NewDeferProcess(ss *ServiceState, delayMs int) *DeferProcess {
	return &DeferProcess{
		parent:  ss,
		delayMs: delayMs,
	}
}

func (dp *DeferProcess) ScheduleRun(run func()) {
	needSchedule := false
	func() {
		dp.mu.Lock()
		defer dp.mu.Unlock()
		if !dp.inFlight {
			dp.inFlight = true
			needSchedule = true
		}
	}()
	if !needSchedule {
		return
	}
	kcommon.ScheduleRun(dp.delayMs, func() {
		dp.parent.PostEvent(DeferEvent{dp: dp, run: run})
	})
}

// DeferEvent: implement IEvent
type DeferEvent struct {
	dp  *DeferProcess
	run func()
}

func (de DeferEvent) GetName() string {
	return "DeferEvent"
}

func (de DeferEvent) Process(ctx context.Context, _ *ServiceState) {
	func() {
		de.dp.mu.Lock()
		defer de.dp.mu.Unlock()
		de.dp.inFlight = false
	}()
	de.run()
}
