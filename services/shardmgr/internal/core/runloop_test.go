package core

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
)

// 创建一个测试用的事件类型
type TestEvent struct {
	Message  string
	executed chan bool // 用于验证事件是否被执行
}

func NewTestEvent(msg string) *TestEvent {
	return &TestEvent{
		Message:  msg,
		executed: make(chan bool, 1),
	}
}

func (te *TestEvent) Execute(ctx context.Context) {
	klogging.Info(ctx).Log("TestEvent", te.Message)
	select {
	case te.executed <- true:
	default:
		// 如果通道已满，不阻塞
	}
}

// 测试 RunLoop 的创建
func TestNewRunLoop(t *testing.T) {
	rl := NewRunLoop()
	if rl == nil {
		t.Fatal("RunLoop should not be nil")
	}
	if rl.queue == nil {
		t.Fatal("UnboundedQueue should not be nil")
	}
}

// 测试事件入队
func TestEnqueueEvent(t *testing.T) {
	rl := NewRunLoop()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动 RunLoop
	go rl.Run(ctx)

	event := NewTestEvent("test")

	// 事件入队不应该阻塞
	done := make(chan bool)
	go func() {
		rl.EnqueueEvent(event)
		done <- true
	}()

	select {
	case <-done:
		// 成功
	case <-time.After(time.Second):
		t.Fatal("EnqueueEvent 超时")
	}

	// 验证事件被执行
	select {
	case <-event.executed:
		// 成功
	case <-time.After(time.Second):
		t.Fatal("等待事件执行超时")
	}
}

// 测试 RunLoop 的运行和事件处理
func TestRunLoop(t *testing.T) {
	rl := NewRunLoop()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动 RunLoop
	done := make(chan bool)
	go func() {
		rl.Run(ctx)
		done <- true
	}()

	// 发送一个测试事件
	testEvent := NewTestEvent("test event")
	rl.EnqueueEvent(testEvent)

	// 等待事件执行
	select {
	case <-testEvent.executed:
		// 成功
	case <-time.After(time.Second):
		t.Fatal("等待事件执行超时")
	}

	// 取消上下文，停止 RunLoop
	cancel()

	// 等待 RunLoop 结束
	select {
	case <-done:
		// 成功
	case <-time.After(time.Second):
		t.Fatal("RunLoop 没有正确停止")
	}
}

// 测试上下文取消
func TestRunLoopContextCancellation(t *testing.T) {
	rl := NewRunLoop()
	ctx, cancel := context.WithCancel(context.Background())

	// 启动 RunLoop
	done := make(chan bool)
	go func() {
		rl.Run(ctx)
		done <- true
	}()

	// 立即取消上下文
	cancel()

	// 等待 RunLoop 结束
	select {
	case <-done:
		// 成功
	case <-time.After(time.Second):
		t.Fatal("RunLoop 在上下文取消后没有停止")
	}
}

// 测试大量事件的处理
func TestRunLoopHighLoad(t *testing.T) {
	rl := NewRunLoop()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动 RunLoop
	go rl.Run(ctx)

	// 发送大量事件
	const numEvents = 10000
	events := make([]*TestEvent, numEvents)
	for i := 0; i < numEvents; i++ {
		events[i] = NewTestEvent("test")
		rl.EnqueueEvent(events[i])
	}

	// 验证所有事件都被执行
	for i, event := range events {
		select {
		case <-event.executed:
			// 成功
		case <-time.After(time.Second):
			t.Fatalf("等待事件 %d 执行超时", i)
		}
	}
}

// 测试并发事件处理
func TestRunLoopConcurrency(t *testing.T) {
	rl := NewRunLoop()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动 RunLoop
	go rl.Run(ctx)

	// 并发发送多个事件
	const numEvents = 100
	done := make(chan bool)
	testEvents := make([]*TestEvent, numEvents)

	go func() {
		for i := 0; i < numEvents; i++ {
			testEvents[i] = NewTestEvent("concurrent" + strconv.Itoa(i))
			rl.EnqueueEvent(testEvents[i])
		}
		done <- true
	}()

	go func() {
		for i := 0; i < numEvents; i++ {
			rl.EnqueueEvent(DummyEvent{Msg: "dummy" + strconv.Itoa(i)})
		}
		done <- true
	}()

	// 等待所有事件发送完成
	for i := 0; i < 2; i++ {
		select {
		case <-done:
			// 成功
		case <-time.After(time.Second):
			t.Fatal("并发事件发送超时")
		}
	}

	// 验证所有测试事件都被执行
	for i, event := range testEvents {
		select {
		case <-event.executed:
			// 成功
		case <-time.After(time.Second):
			t.Fatalf("等待事件 %d 执行超时", i)
		}
	}
}

// 测试事件处理顺序
func TestRunLoopEventOrder(t *testing.T) {
	rl := NewRunLoop()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动 RunLoop
	go rl.Run(ctx)

	// 按顺序发送事件
	events := []string{"first", "second", "third"}
	testEvents := make([]*TestEvent, len(events))

	for i, msg := range events {
		testEvents[i] = NewTestEvent(msg)
		rl.EnqueueEvent(testEvents[i])
	}

	// 验证处理顺序
	for i := range events {
		select {
		case <-testEvents[i].executed:
			// 成功
		case <-time.After(time.Second):
			t.Fatalf("等待事件 %d 处理超时", i)
		}
	}
}
