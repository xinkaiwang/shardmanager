package kcommon

import (
	"context"
	"testing"
	"time"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
)

// 计数器 > 0 时虚拟时钟必须冻结；归零后才允许跳钟。
// （100µs 只是轮询间隔，冻结保证来自计数器条件，与机器负载无关。）
func TestVirtualTimeForward_FrozenWhileBusy(t *testing.T) {
	provider := NewFakeTimeProvider(1000)
	InFlightWorkAdd()

	done := make(chan struct{})
	go func() {
		provider.VirtualTimeForward(context.Background(), 10)
		close(done)
	}()

	// busy 期间：无论真实时间过去多久，虚拟钟都不得前进
	time.Sleep(2 * time.Millisecond) // 20 个轮询周期的真实时间
	if vt := provider.GetMonoTimeMs(); vt != 1000 {
		t.Fatalf("virtual time must be frozen while in-flight work exists, got %d", vt)
	}

	InFlightWorkDone()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("VirtualTimeForward did not complete after work drained")
	}
	if vt := provider.GetMonoTimeMs(); vt != 1010 {
		t.Errorf("virtual time should reach 1010 after drain, got %d", vt)
	}
}

// 深度死锁保护：计数器永不归零时 VTF 必须响亮失败（panic + 现场信息），
// 而不是无限挂起、也不是静默返回让测试在几秒后的下游断言上失败。
func TestVirtualTimeForward_BusyDeadlockPanics(t *testing.T) {
	provider := NewFakeTimeProvider(0)
	InFlightWorkAdd()
	defer InFlightWorkDone()

	done := make(chan *kerror.Kerror)
	go func() {
		defer func() {
			r := recover()
			ke, _ := r.(*kerror.Kerror)
			done <- ke
		}()
		provider.VirtualTimeForward(context.Background(), 5)
	}()
	select {
	case ke := <-done:
		if ke == nil {
			t.Fatal("deadlocked in-flight work must panic with *kerror.Kerror")
		}
		if ke.Type != "FakeTimeInFlightStuck" {
			t.Fatalf("unexpected kerror type %q", ke.Type)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("VTF hung instead of giving up on busy deadlock")
	}
}
