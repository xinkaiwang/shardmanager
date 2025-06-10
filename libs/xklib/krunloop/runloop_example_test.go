package krunloop

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
)

// Example of using RunLoop with a custom resource type

// ExampleResource: implement CriticalResource
type ExampleResource struct {
	Name  string
	Value int
}

// IsResource implements the CriticalResource interface
func (er *ExampleResource) IsResource() {}

// ExampleEvent is an event that works with ExampleResource
type ExampleEvent struct {
	// CreateTimeMs is the time when the event was created
	CreateTimeMs int64 // time when the event was created
	Action       string
	executed     chan bool
}

func (e *ExampleEvent) GetCreateTimeMs() int64 {
	return e.CreateTimeMs
}
func (e *ExampleEvent) GetName() string {
	return "ExampleEvent"
}

func (e *ExampleEvent) Process(ctx context.Context, resource *ExampleResource) {
	// Process the event using the custom resource
	resource.Value += 1
	klogging.Info(ctx).With("action", e.Action).
		With("resource", resource.Name).
		With("new_value", resource.Value).
		Log("ExampleEvent", "Processed example event")

	// Signal that the event was executed
	select {
	case e.executed <- true:
	default:
	}
}

func NewExampleEvent(action string) *ExampleEvent {
	return &ExampleEvent{
		CreateTimeMs: kcommon.GetWallTimeMs(),
		Action:       action,
		executed:     make(chan bool, 1),
	}
}

// TestRunLoopWithExampleResource demonstrates using RunLoop with a custom resource type
func TestRunLoopWithExampleResource(t *testing.T) {
	// Skip in normal test runs
	if testing.Short() {
		t.Skip("Skipping example test in short mode")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a custom resource
	resource := &ExampleResource{
		Name:  "example-resource",
		Value: 0,
	}

	// Create a RunLoop for the custom resource
	rl := NewRunLoop[*ExampleResource](ctx, resource, "test")

	// Start the RunLoop
	done := make(chan bool)
	go func() {
		rl.Run(ctx)
		done <- true
	}()

	// Create and enqueue some events
	events := []*ExampleEvent{
		NewExampleEvent("increment"),
		NewExampleEvent("increment"),
		NewExampleEvent("increment"),
	}

	for _, event := range events {
		rl.PostEvent(event)
	}

	// Wait for all events to be processed
	for _, event := range events {
		select {
		case <-event.executed:
			// Event was executed
		case <-time.After(time.Second):
			t.Fatal("Timed out waiting for event execution")
		}
	}

	// Verify the resource was updated correctly
	if resource.Value != 3 {
		t.Errorf("Resource value = %d, want 3", resource.Value)
	}

	// Clean up
	cancel()
	<-done

	fmt.Println("Example completed successfully")
}
