package ksysmetrics

import (
	"context"
	"time"

	"testing"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"go.opencensus.io/metric"
)

// XS-003：gauge 注册失败（现实中唯一路径 = 未来复制粘贴块忘改名）必须
// 在真正病因处响亮 panic（带 gaugeName 的 kerror），而不是吞错后在下一行
// nil-deref、栈指向错误的位置。
func TestMustRegisterGauge_DuplicateNamePanics(t *testing.T) {
	name := "test_xs003_dup_gauge"
	// 第一次注册成功
	mustRegisterInt64Gauge(name, func() int64 { return 0 },
		metric.WithDescription("test gauge"))

	// 第二次同名注册必须 panic 出带名字的 kerror
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("duplicate gauge name must panic (XS-003)")
		}
		ke, ok := r.(*kerror.Kerror)
		if !ok {
			t.Fatalf("panic value must be *kerror.Kerror, got %T", r)
		}
		if ke.Type != "SysGaugeRegisterFail" {
			t.Errorf("kerror type = %s, want SysGaugeRegisterFail", ke.Type)
		}
	}()
	mustRegisterInt64Gauge(name, func() int64 { return 0 },
		metric.WithDescription("test gauge dup"))
}

// 冷启动窗口：StartSysMetricsCollector 之前只在 ticker 触发时采集，于是进程
// 启动后的第一个 interval 内所有 gauge 读 0——仪表盘上每次重启一段零平台，
// 且与"根本没启动 collector"无法区分（正是 README 记载的无声失败模式）。
// 修法是进 loop 前先同步采一次，本测试钉住它。
func TestStartSysMetricsCollector_CollectsBeforeFirstTick(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// interval 取得远大于测试时长：任何非零值都只可能来自启动时的那次同步采集
	StartSysMetricsCollector(ctx, time.Hour, "test")

	if currentGoroutines <= 0 {
		t.Errorf("goroutine gauge = %d immediately after start, want > 0", currentGoroutines)
	}
	if currentHeapAlloc <= 0 {
		t.Errorf("heap gauge = %d immediately after start, want > 0", currentHeapAlloc)
	}
}
