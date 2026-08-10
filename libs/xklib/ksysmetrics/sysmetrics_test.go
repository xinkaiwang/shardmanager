package ksysmetrics

import (
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
