package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kmetrics"
)

var (
	RunLoopElapsedMsMetric = kmetrics.CreateKmetric(context.Background(), "runloop_elapsed_ms", "desc", []string{"event"})
)

type IEvent interface {
	GetName() string
	Execute(ctx context.Context, ss *ServiceState)
}

// DummyEvent: implement IEvent
type DummyEvent struct {
	Msg string
}

func (de DummyEvent) GetName() string {
	return "DummyEvent"
}

func (de DummyEvent) Execute(ctx context.Context, _ *ServiceState) {
	klogging.Info(ctx).Log("DummyEvent", de.Msg)
}

func NewDummyEvent(msg string) DummyEvent {
	return DummyEvent{Msg: msg}
}

type RunLoop struct {
	ss               *ServiceState
	queue            *UnboundedQueue
	currentEventName string
}

func NewRunLoop(ctx context.Context, ss *ServiceState) *RunLoop {
	rl := &RunLoop{
		ss:    ss,
		queue: NewUnboundedQueue(context.Background()),
	}
	NewRunloopSampler(ctx, func() string { return rl.currentEventName })
	return rl
}

// EnqueueEvent: Enqueue an event to the run loop. This call never blocks.
func (rl *RunLoop) EnqueueEvent(event IEvent) {
	rl.queue.Enqueue(event)
}

func (rl *RunLoop) Run(ctx context.Context) {
	defer rl.queue.Close()

	for {
		select {
		case <-ctx.Done():
			klogging.Info(ctx).Log("RunLoopCtxCanceled", "run loop stopped")
			return
		case event, ok := <-rl.queue.GetOutputChan():
			if !ok {
				klogging.Info(ctx).Log("EventQueueClosed", "event queue closed")
				return
			}
			// Handle event
			start := kcommon.GetMonoTimeMs()
			eveName := event.GetName()
			rl.currentEventName = eveName
			defer func() {
				rl.currentEventName = ""
				elapsedMs := kcommon.GetMonoTimeMs() - start
				RunLoopElapsedMsMetric.GetTimeSequence(ctx, eveName).Add(elapsedMs)
			}()
			event.Execute(ctx, rl.ss)
		}
	}
}
