package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"contrib.go.opencensus.io/exporter/prometheus"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kmetrics"
	"github.com/xinkaiwang/shardmanager/libs/xklib/ksysmetrics"
	"github.com/xinkaiwang/shardmanager/services/hellosvc/internal/biz"
	"github.com/xinkaiwang/shardmanager/services/hellosvc/internal/handler"
	"go.opencensus.io/metric/metricproducer"
)

// 构建时注入的版本信息
var Version string = "dev"       // 通过 -ldflags 注入
var GitCommit string = "unknown" // 通过 -ldflags 注入
var BuildTime string = "unknown" // 通过 -ldflags 注入

// getEnvInt 从环境变量获取整数值，如果不存在或无效则返回默认值
func getEnvInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// curl localhost:8091/api/ping
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
	slogHandler := klogging.NewHandler(&klogging.HandlerOptions{
		Level:  klogging.ParseLevel(logLevel),
		Format: logFormat,
	})
	logger := slog.New(slogHandler)
	slog.SetDefault(logger)
	slog.InfoContext(ctx, "LogLevelSet", slog.String("logLevel", logLevel), slog.String("logFormat", logFormat))

	// 设置版本信息
	biz.SetVersion(Version)
	ksysmetrics.SetVersion(Version) // 设置系统指标的版本号

	// 记录启动信息
	slog.InfoContext(ctx, "Starting hellosvc", slog.String("event", "ServerStarting"), slog.String("version", Version), slog.String("commit", GitCommit), slog.String("buildTime", BuildTime))

	// 创建 Prometheus 导出器
	pe, err := prometheus.NewExporter(prometheus.Options{
		Namespace: "hellosvc",
	})
	if err != nil {
		log.Fatalf("Failed to create Prometheus exporter: %v", err)
	}

	// 注册 kmetrics 注册表
	registry := kmetrics.GetKmetricsRegistry()
	metricproducer.GlobalManager().AddProducer(registry)

	// 注册系统指标注册表
	metricproducer.GlobalManager().AddProducer(ksysmetrics.GetRegistry())

	// 启动系统指标收集器
	ksysmetrics.StartSysMetricsCollector(ctx, 15*time.Second, Version)

	// 获取端口配置
	apiPort := getEnvInt("API_PORT", 8080)
	metricsPort := getEnvInt("METRICS_PORT", 9090)

	// 创建 metrics 路由
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", pe)

	// 创建主路由
	app := biz.NewApp()
	h := handler.NewHandler(app)
	mainMux := http.NewServeMux()
	h.RegisterRoutes(mainMux)

	// 创建主 HTTP 服务器
	mainServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", apiPort),
		Handler: mainMux,
	}

	// 创建 metrics HTTP 服务器
	metricsServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", metricsPort),
		Handler: metricsMux,
	}

	// 记录端口配置
	slog.InfoContext(ctx, "Server ports configuration", slog.String("event", "ServerConfig"), slog.Int("api_port", apiPort), slog.Int("metrics_port", metricsPort))

	// 优雅关闭
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		slog.InfoContext(ctx, "Shutting down servers...", slog.String("event", "ServerShutdown"))
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		if err := mainServer.Shutdown(ctx); err != nil {
			slog.ErrorContext(ctx, "Main server shutdown error", slog.String("event", "MainServerShutdownError"), slog.Any("error", err))
		}
		if err := metricsServer.Shutdown(ctx); err != nil {
			slog.ErrorContext(ctx, "Metrics server shutdown error", slog.String("event", "MetricsServerShutdownError"), slog.Any("error", err))
		}
	}()

	// 启动 metrics 服务器
	go func() {
		slog.InfoContext(ctx, "Metrics server starting", slog.String("event", "MetricsServerStarting"), slog.String("addr", metricsServer.Addr))
		if err := metricsServer.ListenAndServe(); err != http.ErrServerClosed {
			slog.ErrorContext(ctx, "Metrics server error", slog.String("event", "MetricsServerError"), slog.Any("error", err))
		}
	}()

	// 启动主服务器
	slog.InfoContext(ctx, "Main server starting", slog.String("event", "MainServerStarting"), slog.String("addr", mainServer.Addr))
	if err := mainServer.ListenAndServe(); err != http.ErrServerClosed {
		slog.ErrorContext(ctx, "Main server error", slog.String("event", "MainServerError"), slog.Any("error", err))
	}
	slog.InfoContext(ctx, "Servers stopped", slog.String("event", "ServerShutdown"))
}
