package main

import (
	"context"
	"fmt"
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
	"github.com/xinkaiwang/shardmanager/services/etcdmgr/internal/biz"
	"github.com/xinkaiwang/shardmanager/services/etcdmgr/internal/handler"
	"go.opencensus.io/metric/metricproducer"
)

// 构建时注入的版本信息
var (
	Version   = "dev"     // 通过 -ldflags 注入
	GitCommit = "unknown" // 通过 -ldflags 注入
	BuildTime = "unknown" // 通过 -ldflags 注入
)

// getEnvInt 从环境变量获取整数值，如果不存在或无效则返回默认值
func getEnvInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
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

	// 创建并配置 LogrusLogger
	logrusLogger := klogging.NewLogrusLogger(ctx)
	logrusLogger.SetConfig(ctx, logLevel, logFormat)
	klogging.SetDefaultLogger(logrusLogger)
	klogging.Info(ctx).With("logLevel", logLevel).With("logFormat", logFormat).Log("LogLevelSet", "")

	// 设置版本信息
	biz.SetVersion(Version)
	ksysmetrics.SetVersion(Version) // 设置系统指标的版本号

	// 记录启动信息
	klogging.Info(ctx).
		With("version", Version).
		With("commit", GitCommit).
		With("buildTime", BuildTime).
		Log("ServerStarting", "Starting etcdmgr service")

	// 创建 Prometheus 导出器
	pe, err := prometheus.NewExporter(prometheus.Options{
		Namespace: "etcdmgr",
	})
	if err != nil {
		klogging.Fatal(ctx).With("error", err).Log("PrometheusExporterError", "Failed to create Prometheus exporter")
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
	klogging.Info(ctx).
		With("api_port", apiPort).
		With("metrics_port", metricsPort).
		Log("ServerConfig", "Server ports configuration")

	// 优雅关闭
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		klogging.Info(ctx).Log("ServerShutdown", "Shutting down servers...")
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		if err := mainServer.Shutdown(ctx); err != nil {
			klogging.Error(ctx).With("error", err).Log("MainServerShutdownError", "Main server shutdown error")
		}
		if err := metricsServer.Shutdown(ctx); err != nil {
			klogging.Error(ctx).With("error", err).Log("MetricsServerShutdownError", "Metrics server shutdown error")
		}
	}()

	// 启动 metrics 服务器
	go func() {
		klogging.Info(ctx).With("addr", metricsServer.Addr).Log("MetricsServerStarting", "Metrics server starting")
		if err := metricsServer.ListenAndServe(); err != http.ErrServerClosed {
			klogging.Error(ctx).With("error", err).Log("MetricsServerError", "Metrics server error")
		}
	}()

	// 启动主服务器
	klogging.Info(ctx).With("addr", mainServer.Addr).Log("MainServerStarting", "Main server starting")
	if err := mainServer.ListenAndServe(); err != http.ErrServerClosed {
		klogging.Error(ctx).With("error", err).Log("MainServerError", "Main server error")
	}
	klogging.Info(ctx).Log("ServerShutdown", "Servers stopped")
}
