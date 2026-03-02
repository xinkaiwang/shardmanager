package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"contrib.go.opencensus.io/exporter/prometheus"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kmetrics"
	"github.com/xinkaiwang/shardmanager/libs/xklib/ksysmetrics"
	"github.com/xinkaiwang/shardmanager/services/helloblitz/internal/biz"
	"go.opencensus.io/metric/metricproducer"
)

var (
	// 版本信息，通过 -ldflags 注入
	Version   = "dev"
	GitCommit = "none"
	BuildTime = "unknown"
)

func runLoadTest(ctx context.Context, app *biz.UnicornBlitzApp, loopSleepMs int, wg *sync.WaitGroup) {
	defer wg.Done()

	objId := kcommon.RandomString(ctx, 8)
	randomDelay := kcommon.RandomInt(ctx, loopSleepMs)
	slog.InfoContext(ctx, "starting load test",
		slog.String("event", "LoadTestStart"),
		slog.String("objId", objId))
	time.Sleep(time.Duration(randomDelay) * time.Millisecond)
	stop := false
	for !stop {
		select {
		case <-ctx.Done():
			stop = true
			continue
		default:
			ke := kcommon.TryCatchRun(ctx, func() {
				app.RunLoadTest(ctx, objId)
			})
			if ke != nil {
				slog.ErrorContext(ctx, "error during load test",
					slog.String("event", "LoadTestError"),
					slog.String("objId", objId),
					slog.Any("error", ke))
			} else {
				slog.DebugContext(ctx, "load test successful",
					slog.String("event", "LoadTestSuccess"),
					slog.String("objId", objId))
			}

			// 如果需要，在请求之间休眠
			if loopSleepMs > 0 {
				time.Sleep(time.Duration(loopSleepMs) * time.Millisecond)
			}
		}
	}
	slog.InfoContext(ctx, "stopping load test",
		slog.String("event", "LoadTestStop"),
		slog.String("objId", objId))
}

func main() {
	ctx := context.Background()

	// 从环境变量读取日志配置
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info" // 默认日志级别
	}
	logFormat := os.Getenv("LOG_FORMAT")
	if logFormat == "" {
		logFormat = "json" // 默认 JSON 格式
	}

	// Initialize OpenTelemetry
	klogging.InitOpenTelemetry()

	// Create slog handler
	handler := klogging.NewHandler(&klogging.HandlerOptions{
		Level:  klogging.ParseLevel(logLevel),
		Format: logFormat,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)
	slog.InfoContext(ctx, "log level set",
		slog.String("event", "LogLevelSet"),
		slog.String("logLevel", logLevel),
		slog.String("logFormat", logFormat))

	// 记录启动信息
	slog.InfoContext(ctx, "starting helloblitz load test driver",
		slog.String("event", "Starting"))

	// 创建 Prometheus 导出器
	pe, err := prometheus.NewExporter(prometheus.Options{
		Namespace: "helloblitz",
	})
	if err != nil {
		slog.ErrorContext(ctx, "failed to create prometheus exporter",
			slog.String("event", "PrometheusExporterError"),
			slog.Any("error", err))
		os.Exit(1)
	}

	// 注册 kmetrics 注册表
	registry := kmetrics.GetKmetricsRegistry()
	metricproducer.GlobalManager().AddProducer(registry)

	// 注册系统指标注册表
	metricproducer.GlobalManager().AddProducer(ksysmetrics.GetRegistry())

	// 启动系统指标收集器
	ksysmetrics.StartSysMetricsCollector(ctx, 15*time.Second, Version)

	// 获取 metrics 端口配置
	metricsPort := kcommon.GetEnvInt("METRICS_PORT", 9090)

	// 创建 metrics 服务器
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", pe)
	metricsServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", metricsPort),
		Handler: metricsMux,
	}

	// 启动 metrics 服务器
	go func() {
		slog.InfoContext(ctx, "starting metrics server",
			slog.String("event", "MetricsServerStarting"),
			slog.String("addr", metricsServer.Addr))
		if err := metricsServer.ListenAndServe(); err != http.ErrServerClosed {
			slog.ErrorContext(ctx, "metrics server error",
				slog.String("event", "MetricsServerError"),
				slog.Any("error", err))
		}
	}()

	// 获取环境变量配置
	threadCount := kcommon.GetEnvInt("BLITZ_THREAD_COUNT", 3)
	loopSleepMs := kcommon.GetEnvInt("BLITZ_LOOP_SLEEP_MS", 0)

	slog.InfoContext(ctx, "load test configuration",
		slog.String("event", "Config"),
		slog.Int("thread_count", threadCount),
		slog.Int("loop_sleep_ms", loopSleepMs),
		slog.Int("metrics_port", metricsPort))

	app := biz.NewUnicornBlitzApp(ctx)

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 创建取消上下文
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// 启动负载测试线程
	var wg sync.WaitGroup
	for i := 0; i < threadCount; i++ {
		wg.Add(1)
		go runLoadTest(ctx, app, loopSleepMs, &wg)
	}

	// 等待信号
	sig := <-sigChan
	fmt.Printf("\nReceived signal %v, shutting down...\n", sig)

	// 取消上下文，停止所有线程
	cancel()

	// 等待所有线程完成
	wg.Wait()

	// 优雅关闭 metrics 服务器
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		slog.ErrorContext(ctx, "error shutting down metrics server",
			slog.String("event", "MetricsServerShutdownError"),
			slog.Any("error", err))
	}

	slog.InfoContext(ctx, "load test driver stopped",
		slog.String("event", "Shutdown"))
}
