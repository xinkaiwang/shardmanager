package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
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
	klogging.Info(ctx).With("version", Version).With("commit", GitCommit).With("buildTime", BuildTime).Log("ServerStarting", "Starting hellosvc")

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

	// 创建应用实例
	app := biz.NewApp()
	h := handler.NewHandler(app)

	// 创建主路由
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	// 创建 metrics 路由
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", pe)

	// 创建主 HTTP 服务器
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// 创建 metrics HTTP 服务器
	metricsServer := &http.Server{
		Addr:    ":9090",
		Handler: metricsMux,
	}

	// 优雅关闭
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		klogging.Info(ctx).Log("ServerShutdown", "Shutting down servers...")
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
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
	klogging.Info(ctx).With("addr", server.Addr).Log("MainServerStarting", "Main server starting")
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		klogging.Error(ctx).With("error", err).Log("MainServerError", "Main server error")
	}
	klogging.Info(ctx).Log("ServerShutdown", "Servers stopped")
}
