package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"contrib.go.opencensus.io/exporter/prometheus"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kmetrics"
	"github.com/xinkaiwang/shardmanager/libs/xklib/ksysmetrics"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/biz"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/handler"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/solo"
	"go.opencensus.io/metric/metricproducer"
)

func main() {
	ctx := context.Background()
	// 从环境变量读取日志配置
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info" // 默认日志级别
	}
	logFormat := os.Getenv("LOG_FORMAT")
	if logFormat == "" {
		logFormat = "simple" // 默认格式
	}

	// 初始化 OpenTelemetry
	klogging.InitOpenTelemetry()

	// 创建并配置 slog Handler
	slogHandler := klogging.NewHandler(&klogging.HandlerOptions{
		Level:  klogging.ParseLevel(logLevel),
		Format: logFormat,
	})
	logger := slog.New(slogHandler)
	slog.SetDefault(logger)

	slog.InfoContext(ctx, "",
		slog.String("event", "LogLevelSet"),
		slog.String("logLevel", logLevel),
		slog.String("logFormat", logFormat))

	// 设置版本信息
	Version := common.GetVersion()
	ksysmetrics.SetVersion(Version) // 设置系统指标的版本号

	// 记录启动信息
	slog.InfoContext(ctx, "Starting shardmgr",
		slog.String("event", "ServerStarting"),
		slog.String("version", Version),
		slog.String("sessionId", common.GetSessionId()),
		slog.Int64("startTime", common.GetStartTimeMs()))

	// 创建 Prometheus 导出器
	pe, err := prometheus.NewExporter(prometheus.Options{
		Namespace: "shardmgr",
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
	apiPort := common.GetEnvInt("API_PORT", 8081)
	metricsPort := common.GetEnvInt("METRICS_PORT", 9091)
	appName := common.GetEnvStr("SMGAPP_NAME", "smgapp")

	// 创建 metrics 路由
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", pe)

	// 创建业务逻辑层
	slog.InfoContext(ctx, "Creating app instance",
		slog.String("event", "AppStarting"))
	app := biz.NewApp(ctx, appName)
	slog.InfoContext(ctx, "App instance created",
		slog.String("event", "AppCreated"))
	app.StartAppMetrics(ctx)
	metricproducer.GlobalManager().AddProducer(app.GetRegistry())

	// 创建主路由
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

	// solo manager
	podName := os.Getenv("POD_NAME")
	if podName == "" {
		podName = GetHostname()
	}
	soloMgr := solo.NewSoloManagerRobust(ctx, podName)
	go func() {
		<-soloMgr.ChLockLost
		fmt.Println("Lock lost, exiting...")
		os.Exit(1)
	}()

	// 记录端口配置
	slog.InfoContext(ctx, "Server ports configuration",
		slog.String("event", "ServerConfig"),
		slog.Int("api_port", apiPort),
		slog.Int("metrics_port", metricsPort))

	// 优雅关闭
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigChan

		slog.InfoContext(ctx, "Shutting down servers...",
			slog.String("event", "ServerShutdown"),
			slog.Any("signal", sig))
		soloMgr.Close(ctx)
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		if err := mainServer.Shutdown(ctx); err != nil {
			slog.ErrorContext(ctx, "Main server shutdown error",
				slog.String("event", "MainServerShutdownError"),
				slog.Any("error", err))
		}
		if err := metricsServer.Shutdown(ctx); err != nil {
			slog.ErrorContext(ctx, "Metrics server shutdown error",
				slog.String("event", "MetricsServerShutdownError"),
				slog.Any("error", err))
		}
	}()

	// 启动 metrics 服务器
	go func() {
		slog.InfoContext(ctx, "Metrics server starting",
			slog.String("event", "MetricsServerStarting"),
			slog.String("addr", metricsServer.Addr))
		if err := metricsServer.ListenAndServe(); err != http.ErrServerClosed {
			slog.ErrorContext(ctx, "Metrics server error",
				slog.String("event", "MetricsServerError"),
				slog.Any("error", err))
		}
	}()

	// 启动主服务器
	slog.InfoContext(ctx, "Main server starting",
		slog.String("event", "MainServerStarting"),
		slog.String("addr", mainServer.Addr))
	if err := mainServer.ListenAndServe(); err != http.ErrServerClosed {
		slog.ErrorContext(ctx, "Main server error",
			slog.String("event", "MainServerError"),
			slog.Any("error", err))
	}
	slog.InfoContext(ctx, "Servers stopped",
		slog.String("event", "ServerShutdown"))
}

// GetHostname 返回当前主机名
func GetHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		slog.ErrorContext(context.Background(), "Failed to get hostname",
			slog.String("event", "GetHostname"),
			slog.Any("error", err))
		return "unknown"
	}
	return hostname
}
