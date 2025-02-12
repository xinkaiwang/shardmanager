package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"contrib.go.opencensus.io/exporter/prometheus"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kmetrics"
	"github.com/xinkaiwang/shardmanager/services/helloblitz/internal/biz"
	"go.opencensus.io/metric/metricproducer"
)

var (
	// 版本信息，通过 -ldflags 注入
	Version   = "dev"
	GitCommit = "none"
	BuildTime = "unknown"
)

func getEnvInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

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
				klogging.Error(ctx).With("latency_ms", result.LatencyMs).
					With("status_code", result.StatusCode).
					Log("LoadTestError", result.Error)
			} else {
				klogging.Info(ctx).With("latency_ms", result.LatencyMs).
					With("status_code", result.StatusCode).
					Log("LoadTestSuccess", "Request completed successfully")
			}

			// 如果需要，在请求之间休眠
			if loopSleepMs > 0 {
				time.Sleep(time.Duration(loopSleepMs) * time.Millisecond)
			}
		}
	}
}

func main() {
	// 配置日志
	ctx := context.Background()
	klogging.Info(ctx).With("version", Version).
		With("commit", GitCommit).
		With("buildTime", BuildTime).
		Log("Starting", "Starting helloblitz load test driver")

	// 创建 Prometheus 导出器
	pe, err := prometheus.NewExporter(prometheus.Options{
		Namespace: "helloblitz",
	})
	if err != nil {
		klogging.Fatal(ctx).With("error", err).Log("PrometheusExporterError", "Failed to create Prometheus exporter")
	}

	// 注册 kmetrics 注册表
	registry := kmetrics.GetKmetricsRegistry()
	metricproducer.GlobalManager().AddProducer(registry)

	// 创建 metrics 服务器
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", pe)
	metricsServer := &http.Server{
		Addr:    ":9090",
		Handler: metricsMux,
	}

	// 启动 metrics 服务器
	go func() {
		klogging.Info(ctx).With("addr", metricsServer.Addr).Log("MetricsServerStarting", "Starting metrics server")
		if err := metricsServer.ListenAndServe(); err != http.ErrServerClosed {
			klogging.Error(ctx).With("error", err).Log("MetricsServerError", "Metrics server error")
		}
	}()

	// 获取环境变量配置
	threadCount := getEnvInt("BLITZ_THREAD_COUNT", 3)
	loopSleepMs := getEnvInt("BLITZ_LOOP_SLEEP_MS", 0)
	targetURL := os.Getenv("BLITZ_TARGET_URL")
	if targetURL == "" {
		targetURL = "http://localhost:8080/api/ping"
	}

	klogging.Info(ctx).With("thread_count", threadCount).
		With("loop_sleep_ms", loopSleepMs).
		With("target_url", targetURL).
		Log("Config", "Load test configuration")

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
		klogging.Error(ctx).With("error", err).Log("MetricsServerShutdownError", "Error shutting down metrics server")
	}

	klogging.Info(ctx).Log("Shutdown", "Load test driver stopped")
}
