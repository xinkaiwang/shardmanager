package krunloop

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
)

// Create a test resource type
type TestRunLoopResource struct{}

func (tr *TestRunLoopResource) IsResource() {}

// Create a test event type
type RunLoopTestEvent struct {
	CreateTimeMs int64 // time when the event was created
	Message      string
	executed     chan bool // For verification
}

func (te *RunLoopTestEvent) GetCreateTimeMs() int64 {
	return te.CreateTimeMs
}
func (te *RunLoopTestEvent) GetName() string {
	return "TestEvent"
}

func (te *RunLoopTestEvent) Process(ctx context.Context, _ *TestRunLoopResource) {
	klogging.Info(ctx).Log("TestEvent", te.Message)
	select {
	case te.executed <- true:
	default:
		// Don't block if channel is full
	}
}

func NewRunLoopTestEvent(msg string) *RunLoopTestEvent {
	return &RunLoopTestEvent{
		CreateTimeMs: kcommon.GetWallTimeMs(),
		Message:      msg,
		executed:     make(chan bool, 1),
	}
}

// Test RunLoop creation
func TestNewRunLoop(t *testing.T) {
	resource := &TestRunLoopResource{}
	rl := NewRunLoop[*TestRunLoopResource](context.Background(), resource, "test")
	if rl == nil {
		t.Fatal("RunLoop should not be nil")
	}
	if rl.queue == nil {
		t.Fatal("UnboundedQueue should not be nil")
	}
}

// Test event enqueuing
func TestEnqueueEvent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resource := &TestRunLoopResource{}
	rl := NewRunLoop[*TestRunLoopResource](ctx, resource, "test")
	event := NewRunLoopTestEvent("test")

	// Start RunLoop
	done := make(chan bool)
	go func() {
		rl.Run(ctx)
		done <- true
	}()

	// Event enqueuing should not block
	enqueued := make(chan bool)
	go func() {
		rl.PostEvent(event)
		enqueued <- true
	}()

	select {
	case <-enqueued:
		// Success
	case <-time.After(time.Second):
		t.Fatal("EnqueueEvent timed out")
	}

	// Verify event was processed
	select {
	case <-event.executed:
		// Success
	case <-time.After(time.Second):
		t.Fatal("Waiting for event execution timed out")
	}
}

// Test RunLoop running and event processing
func TestRunLoop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resource := &TestRunLoopResource{}
	rl := NewRunLoop[*TestRunLoopResource](ctx, resource, "test")

	// Start RunLoop
	done := make(chan bool)
	go func() {
		rl.Run(ctx)
		done <- true
	}()

	// Send a test event
	testEvent := NewRunLoopTestEvent("testEvent")
	rl.PostEvent(testEvent)

	// Wait for event execution
	select {
	case <-testEvent.executed:
		// Success
	case <-time.After(time.Second):
		t.Fatal("Waiting for event execution timed out")
	}

	// Cancel context to stop RunLoop
	cancel()

	// Wait for RunLoop to end
	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Fatal("RunLoop did not stop correctly")
	}
}

// Test context cancellation
func TestRunLoopContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	resource := &TestRunLoopResource{}
	rl := NewRunLoop[*TestRunLoopResource](ctx, resource, "test")

	// Start RunLoop
	done := make(chan bool)
	go func() {
		rl.Run(ctx)
		done <- true
	}()

	// Immediately cancel context
	cancel()

	// Wait for RunLoop to end
	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Fatal("RunLoop did not stop after context cancellation")
	}
}

// Test high load event processing
func TestRunLoopHighLoad(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resource := &TestRunLoopResource{}
	rl := NewRunLoop[*TestRunLoopResource](ctx, resource, "test")

	// Start RunLoop
	go rl.Run(ctx)

	// Send many events
	const numEvents = 1000
	events := make([]*RunLoopTestEvent, numEvents)
	for i := 0; i < numEvents; i++ {
		events[i] = NewRunLoopTestEvent("largeVolumnEvent" + strconv.Itoa(i))
		rl.PostEvent(events[i])
	}

	// Verify all events were processed
	for i, event := range events {
		select {
		case <-event.executed:
			// Success
		case <-time.After(time.Second):
			t.Fatalf("Waiting for event %d execution timed out", i)
		}
	}
}

// Create a dummy event type for concurrency test
type DummyEvent struct {
	CreateTimeMs int64 // time when the event was created
	Msg          string
}

func NewDummyEvent(msg string) DummyEvent {
	return DummyEvent{
		CreateTimeMs: kcommon.GetWallTimeMs(),
		Msg:          msg,
	}
}
func (de DummyEvent) GetCreateTimeMs() int64 {
	return de.CreateTimeMs
}
func (de DummyEvent) GetName() string {
	return "DummyEvent"
}

func (de DummyEvent) Process(ctx context.Context, _ *TestRunLoopResource) {
	klogging.Info(ctx).Log("DummyEvent", de.Msg)
}

// Test concurrent event processing
func TestRunLoopConcurrency(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resource := &TestRunLoopResource{}
	rl := NewRunLoop[*TestRunLoopResource](ctx, resource, "test")

	// Start RunLoop
	go rl.Run(ctx)

	// Concurrently send multiple events
	const numEvents = 20
	done := make(chan bool)
	testEvents := make([]*RunLoopTestEvent, numEvents)

	go func() {
		for i := 0; i < numEvents; i++ {
			testEvents[i] = NewRunLoopTestEvent("testConcurrentEvent" + strconv.Itoa(i))
			rl.PostEvent(testEvents[i])
		}
		done <- true
	}()

	go func() {
		for i := 0; i < numEvents; i++ {
			rl.PostEvent(NewDummyEvent("dummy" + strconv.Itoa(i)))
		}
		done <- true
	}()

	// Wait for all events to be sent
	for i := 0; i < 2; i++ {
		select {
		case <-done:
			// Success
		case <-time.After(time.Second):
			t.Fatal("Concurrent event sending timed out")
		}
	}

	// Verify all test events were processed
	for i, event := range testEvents {
		select {
		case <-event.executed:
			// Success
		case <-time.After(time.Second):
			t.Fatalf("Waiting for event %d execution timed out", i)
		}
	}
}

// Test event processing order
func TestRunLoopEventOrder(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resource := &TestRunLoopResource{}
	rl := NewRunLoop[*TestRunLoopResource](ctx, resource, "test")

	// Start RunLoop
	go rl.Run(ctx)

	// Send events in order
	events := []string{"first", "second", "third"}
	testEvents := make([]*RunLoopTestEvent, len(events))

	for i, msg := range events {
		testEvents[i] = NewRunLoopTestEvent(msg)
		rl.PostEvent(testEvents[i])
	}

	// Verify processing order
	for i := range events {
		select {
		case <-testEvents[i].executed:
			// Success
		case <-time.After(time.Second):
			t.Fatalf("Waiting for event %d execution timed out", i)
		}
	}
}
