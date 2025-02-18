package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
)

type IEvent interface {
	Execute(ctx context.Context)
}

// DummyEvent: implement IEvent
type DummyEvent struct {
	Msg string
}

func (de DummyEvent) Execute(ctx context.Context) {
	klogging.Info(ctx).Log("DummyEvent", de.Msg)
}

func NewDummyEvent(msg string) DummyEvent {
	return DummyEvent{Msg: msg}
}

type RunLoop struct {
	queue *UnboundedQueue
}

func NewRunLoop() *RunLoop {
	return &RunLoop{
		queue: NewUnboundedQueue(context.Background()),
	}
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
			event.Execute(ctx)
		}
	}
}
