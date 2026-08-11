package kcommon

import "sync/atomic"

// In-flight work counter：虚拟时间测试的静默（quiescence）信号。
//
// 语义：计数"已投递但尚未处理完"的异步工作单元。FakeTimeProvider 在计数
// 归零前冻结虚拟时钟——把旧实现"睡 100µs 赌系统安静了"的猜测，换成精确
// 条件（2026-08-10 设计，见 research/2026-08-10-fake-time-quiescence/notes.md）。
//
// 记账方（目前唯一）：krunloop——Enqueue +1，事件 Process 返回后 -1
// （减数必须在 Process 之后：保证 handler 内的 ScheduleRun 先入任务堆、
// 计数才归零，跳钟时堆是完整的）。queue 停止时未消费的事件由 run() 的
// defer 批量补减。
//
// 生产开销：每事件两次原子加，纳秒级；不按 provider 类型分支（分支比白加贵）。
var inFlightWork atomic.Int64

func InFlightWorkAdd() { inFlightWork.Add(1) }

func InFlightWorkDone() { inFlightWork.Add(-1) }

func InFlightWorkCount() int64 { return inFlightWork.Load() }
