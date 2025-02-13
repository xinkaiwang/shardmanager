package ksysmetrics

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"syscall"
	"time"

	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"go.opencensus.io/metric"
	"go.opencensus.io/metric/metricdata"
)

var (
	registry *metric.Registry
	// CPU 指标
	userCPUGauge   *metric.Float64DerivedGauge
	systemCPUGauge *metric.Float64DerivedGauge

	// 内存指标
	heapAllocGauge  *metric.Int64DerivedGauge
	stackInuseGauge *metric.Int64DerivedGauge
	sysMemGauge     *metric.Int64DerivedGauge

	// Goroutine 指标
	goroutineGauge *metric.Int64DerivedGauge

	// 文件描述符指标
	fdGauge *metric.Int64DerivedGauge

	// GC 指标
	gcPauseGauge       *metric.Int64DerivedGauge
	gcIntervalGauge    *metric.Int64DerivedGauge
	gcCPUFractionGauge *metric.Float64DerivedGauge

	// 当前值
	currentUserCPU       float64
	currentSystemCPU     float64
	currentHeapAlloc     int64
	currentStackInuse    int64
	currentSysMem        int64
	currentGoroutines    int64
	currentFDs           int64
	currentGCPause       int64
	currentGCInterval    int64
	currentGCCPUFraction float64

	// 版本信息
	currentVersion = "unknown"
)

// SetVersion 设置服务版本号
func SetVersion(version string) {
	if version != "" {
		currentVersion = version
	}
}

func init() {
	registry = metric.NewRegistry()

	// 创建 CPU 指标
	userCPUGauge, _ = registry.AddFloat64DerivedGauge(
		"process_user_cpu_seconds",
		metric.WithDescription("User CPU time spent in seconds"),
		metric.WithUnit("seconds"))
	userCPUGauge.UpsertEntry(func() float64 { return currentUserCPU })

	systemCPUGauge, _ = registry.AddFloat64DerivedGauge(
		"process_system_cpu_seconds",
		metric.WithDescription("System CPU time spent in seconds"),
		metric.WithUnit("seconds"))
	systemCPUGauge.UpsertEntry(func() float64 { return currentSystemCPU })

	// 创建内存指标
	heapAllocGauge, _ = registry.AddInt64DerivedGauge(
		"process_heap_bytes",
		metric.WithDescription("Process heap memory in bytes"),
		metric.WithUnit("bytes"))
	heapAllocGauge.UpsertEntry(func() int64 { return currentHeapAlloc })

	stackInuseGauge, _ = registry.AddInt64DerivedGauge(
		"process_stack_bytes",
		metric.WithDescription("Process stack memory in bytes"),
		metric.WithUnit("bytes"))
	stackInuseGauge.UpsertEntry(func() int64 { return currentStackInuse })

	sysMemGauge, _ = registry.AddInt64DerivedGauge(
		"process_resident_memory_bytes",
		metric.WithDescription("Resident memory size in bytes"),
		metric.WithUnit("bytes"))
	sysMemGauge.UpsertEntry(func() int64 { return currentSysMem })

	// 创建 Goroutine 指标
	goroutineGauge, _ = registry.AddInt64DerivedGauge(
		"process_goroutines",
		metric.WithDescription("Number of goroutines"))
	goroutineGauge.UpsertEntry(func() int64 { return currentGoroutines })

	// 创建文件描述符指标
	fdGauge, _ = registry.AddInt64DerivedGauge(
		"process_open_fds",
		metric.WithDescription("Number of open file descriptors"))
	fdGauge.UpsertEntry(func() int64 { return currentFDs })

	// 创建 GC 指标
	gcPauseGauge, _ = registry.AddInt64DerivedGauge(
		"process_gc_pause_total_ns",
		metric.WithDescription("Total GC pause time in nanoseconds"),
		metric.WithUnit("ns"))
	gcPauseGauge.UpsertEntry(func() int64 { return currentGCPause })

	gcIntervalGauge, _ = registry.AddInt64DerivedGauge(
		"process_gc_interval_ms",
		metric.WithDescription("Time since last GC in milliseconds"),
		metric.WithUnit("ms"))
	gcIntervalGauge.UpsertEntry(func() int64 { return currentGCInterval })

	gcCPUFractionGauge, _ = registry.AddFloat64DerivedGauge(
		"process_gc_cpu_fraction",
		metric.WithDescription("Fraction of CPU time used by GC"))
	gcCPUFractionGauge.UpsertEntry(func() float64 { return currentGCCPUFraction })
}

// StartSysMetricsCollector 启动系统指标收集器
// interval: 收集间隔时间
// version: 服务版本号，将被添加到 CPU 指标的标签中
func StartSysMetricsCollector(ctx context.Context, interval time.Duration, version string) {
	// 设置版本号
	if version != "" {
		currentVersion = version
	}

	// 更新 CPU 指标的版本标签
	userCPUGauge, _ = registry.AddFloat64DerivedGauge(
		"process_user_cpu_seconds",
		metric.WithDescription("User CPU time spent in seconds"),
		metric.WithUnit("seconds"),
		metric.WithLabelKeys("version"))
	userCPUGauge.UpsertEntry(func() float64 { return currentUserCPU }, metricdata.NewLabelValue(currentVersion))

	systemCPUGauge, _ = registry.AddFloat64DerivedGauge(
		"process_system_cpu_seconds",
		metric.WithDescription("System CPU time spent in seconds"),
		metric.WithUnit("seconds"),
		metric.WithLabelKeys("version"))
	systemCPUGauge.UpsertEntry(func() float64 { return currentSystemCPU }, metricdata.NewLabelValue(currentVersion))

	// 启动定时收集
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		// 获取进程 ID
		pid := os.Getpid()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				collectMetrics(ctx, pid)
			}
		}
	}()
}

func collectMetrics(ctx context.Context, pid int) {
	// 收集 CPU 使用率
	var rusage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &rusage); err == nil {
		userCPU := time.Duration(rusage.Utime.Sec)*time.Second + time.Duration(rusage.Utime.Usec)*time.Microsecond
		sysCPU := time.Duration(rusage.Stime.Sec)*time.Second + time.Duration(rusage.Stime.Usec)*time.Microsecond

		currentUserCPU = userCPU.Seconds()
		currentSystemCPU = sysCPU.Seconds()
	} else {
		klogging.Error(ctx).With("error", err).Log("CPUMetricsError", "Failed to collect CPU metrics")
	}

	// 收集内存使用情况
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	currentHeapAlloc = int64(memStats.HeapAlloc)
	currentStackInuse = int64(memStats.StackInuse)
	currentSysMem = int64(memStats.Sys)

	// 收集 Goroutine 数量
	currentGoroutines = int64(runtime.NumGoroutine())

	// 收集文件描述符数量
	if fds, err := getFDCount(pid); err == nil {
		currentFDs = int64(fds)
	} else {
		klogging.Error(ctx).With("error", err).Log("FDMetricsError", "Failed to collect FD metrics")
	}

	// 收集 GC 统计信息
	currentGCPause = int64(memStats.PauseTotalNs)
	currentGCInterval = int64(memStats.LastGC / 1e6) // 转换为毫秒
	currentGCCPUFraction = memStats.GCCPUFraction
}

// getFDCount 获取进程打开的文件描述符数量
func getFDCount(pid int) (int, error) {
	fdPath := fmt.Sprintf("/proc/%d/fd", pid)
	fds, err := os.ReadDir(fdPath)
	if err != nil {
		return 0, err
	}
	return len(fds), nil
}

// GetRegistry 返回指标注册表
func GetRegistry() *metric.Registry {
	return registry
}
