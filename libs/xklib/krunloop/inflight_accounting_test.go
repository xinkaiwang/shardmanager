package krunloop

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
)

// gatedEvent 的 Process 阻塞在 gate 上，用于制造"已入队未处理完"的确定性状态
type gatedEvent struct {
	gate *sync.WaitGroup // Process 等它
	done *sync.WaitGroup // Process 完成时 Done
}

func (e *gatedEvent) GetCreateTimeMs() int64 { return 0 }
func (e *gatedEvent) GetName() string        { return "Gated" }
func (e *gatedEvent) Process(ctx context.Context, _ *TestRunLoopResource) {
	e.gate.Wait()
	e.done.Done()
}

func waitCount(t *testing.T, want int64, msg string) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		if kcommon.InFlightWorkCount() == want {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("%s: in-flight=%d, want %d", msg, kcommon.InFlightWorkCount(), want)
		case <-time.After(time.Millisecond):
		}
	}
}

// 全生命周期记账：入队 +1，处理完 -1，全部处理后归零
func TestInFlightAccounting_DrainsToZero(t *testing.T) {
	base := kcommon.InFlightWorkCount()
	ctx := context.Background()
	rl := NewRunLoop(ctx, &TestRunLoopResource{}, "acct-loop")
	go rl.Run(ctx)

	var gate, done sync.WaitGroup
	gate.Add(1)
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(gate.Done) }
	// LIFO：release 先于 StopAndWaitForExit 执行——测试中途 Fatal 时必须先放行
	// gate，否则 StopAndWaitForExit（永远等待在途事件）会把测试挂死
	defer rl.StopAndWaitForExit()
	defer release()

	done.Add(3)
	for i := 0; i < 3; i++ {
		rl.PostEvent(&gatedEvent{gate: &gate, done: &done})
	}
	waitCount(t, base+3, "3 events posted, none processed")

	release() // 放行
	done.Wait()
	waitCount(t, base, "all processed")
}

// 停机配平：queue 停止时 buffer 里未消费的事件必须补减，计数器不得永久污染
func TestInFlightAccounting_StopWithBufferedEvents(t *testing.T) {
	base := kcommon.InFlightWorkCount()
	q := NewUnboundedQueue[*TestRunLoopResource](context.Background())

	// 无消费者：3 个事件滞留在 queue 内部
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		q.Enqueue(&gatedEvent{gate: &wg, done: &wg})
	}
	waitCount(t, base+3, "3 events buffered")

	q.StopAndWaitForExit()
	waitCount(t, base, "buffered events reclaimed on stop")

	// 停机后的掉队投递：响亮丢弃，且不得改变计数
	q.Enqueue(&gatedEvent{gate: &wg, done: &wg})
	waitCount(t, base, "post-stop enqueue must not leak the counter")
}
