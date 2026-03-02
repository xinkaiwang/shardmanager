package main

import (
	"context"
	"fmt"
	"log/slog" // Added slog
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"contrib.go.opencensus.io/exporter/prometheus"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging" // Retained for InitOpenTelemetry, NewHandler, ParseLevel
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

func runLoadTest(ctx context.Context, app *biz.App, targetURL string, loopSleepMs int, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			result := app.RunLoadTest(ctx, targetURL)

			// 记录结果
			if result.Error != "" {
				slog.ErrorContext(ctx, result.Error,
					slog.String("event", "LoadTestError"),
					slog.Float64("latency_ms", result.LatencyMs),
					slog.Int("status_code", result.StatusCode))
			} else {
				slog.InfoContext(ctx, "Request completed successfully",
					slog.String("event", "LoadTestSuccess"),
					slog.Float64("latency_ms", result.LatencyMs),
					slog.Int("status_code", result.StatusCode))
			}

			// 如果需要，在请求之间休眠
			if loopSleepMs > 0 {
				time.Sleep(time.Duration(loopSleepMs) * time.Millisecond)
			}
		}
	}
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
	slog.InfoContext(ctx, "", slog.String("event", "LogLevelSet"), slog.String("logLevel", logLevel), slog.String("logFormat", logFormat))

	// 记录启动信息
	slog.InfoContext(ctx, "Starting helloblitz load test driver",
		slog.String("event", "Starting"),
		slog.String("version", Version),
		slog.String("commit", GitCommit),
		slog.String("buildTime", BuildTime))

	// 创建 Prometheus 导出器
	pe, err := prometheus.NewExporter(prometheus.Options{
		Namespace: "helloblitz",
	})
	if err != nil {
		slog.ErrorContext(ctx, "Failed to create Prometheus exporter",
			slog.String("event", "PrometheusExporterError"),
			slog.Any("error", err))
		os.Exit(1) // Added os.Exit(1) for Fatal
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
		slog.InfoContext(ctx, "Starting metrics server", slog.String("event", "MetricsServerStarting"), slog.String("addr", metricsServer.Addr))
		if err := metricsServer.ListenAndServe(); err != http.ErrServerClosed {
			slog.ErrorContext(ctx, "Metrics server error", slog.String("event", "MetricsServerError"), slog.Any("error", err))
		}
	}()

	// 获取环境变量配置
	threadCount := kcommon.GetEnvInt("BLITZ_THREAD_COUNT", 3)
	loopSleepMs := kcommon.GetEnvInt("BLITZ_LOOP_SLEEP_MS", 0)
	targetURL := os.Getenv("BLITZ_TARGET_URL")
	if targetURL == "" {
		targetURL = "http://localhost:8080/api/ping"
	}

	slog.InfoContext(ctx, "Load test configuration",
		slog.String("event", "Config"),
		slog.Int("thread_count", threadCount),
		slog.Int("loop_sleep_ms", loopSleepMs),
		slog.String("target_url", targetURL),
		slog.Int("metrics_port", metricsPort))

	// 创建应用实例
	app := biz.NewApp()

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
		go runLoadTest(ctx, app, targetURL, loopSleepMs, &wg)
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
		slog.ErrorContext(ctx, "Error shutting down metrics server", slog.String("event", "MetricsServerShutdownError"), slog.Any("error", err))
	}

	slog.InfoContext(ctx, "Load test driver stopped", slog.String("event", "Shutdown"))
}
