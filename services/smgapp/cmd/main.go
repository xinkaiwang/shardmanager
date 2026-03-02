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
	"github.com/xinkaiwang/shardmanager/libs/cougar/cougar"
	"github.com/xinkaiwang/shardmanager/libs/unicorn/data"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kmetrics"
	"github.com/xinkaiwang/shardmanager/libs/xklib/ksysmetrics"
	"github.com/xinkaiwang/shardmanager/services/smgapp/internal/biz"
	"github.com/xinkaiwang/shardmanager/services/smgapp/internal/handler"
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

/*
export POD_IP=127.0.0.1
export POD_NAME=worker-1
export SMG_CAPACITY=1000
export SMG_STORAGE_SIZE_MB=8000
export API_PORT=8082
export METRICS_PORT=9092
export LOG_LEVEL=info
export LOG_FORMAT=json
./bin/smgapp
*/
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

	// 初始化 OpenTelemetry
	klogging.InitOpenTelemetry()
	
	// 创建并配置 slog Handler
	slogHandler := klogging.NewHandler(&klogging.HandlerOptions{
		Level:  klogging.ParseLevel(logLevel),
		Format: logFormat,
	})
	logger := slog.New(slogHandler)
	slog.SetDefault(logger)
	
	slog.InfoContext(ctx, "Logger configured",
		slog.String("event", "LogLevelSet"),
		slog.String("logLevel", logLevel),
		slog.String("logFormat", logFormat),
	)

	// 设置版本信息
	biz.SetVersion(Version)
	ksysmetrics.SetVersion(Version) // 设置系统指标的版本号

	// 记录启动信息
	slog.InfoContext(ctx, "Starting hellosvc",
		slog.String("event", "ServerStarting"),
		slog.String("version", Version),
		slog.String("commit", GitCommit),
		slog.String("buildTime", BuildTime),
	)

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

	// workerInfo
	workerName := os.Getenv("POD_NAME")
	if workerName == "" {
		ke := kerror.Create("POD_NAME not set", "")
		panic(ke)
	}
	sessionId := kcommon.RandomString(ctx, 8)
	address := kcommon.GetEnvString("POD_IP", "localhost")
	capacity := getEnvInt("SMG_CAPACITY", 1000)
	memorySize := getEnvInt("SMG_STORAGE_SIZE_MB", 8*1000)
	addrPort := fmt.Sprintf("%s:%d", address, apiPort)
	workerInfo := cougar.NewWorkerInfo(data.WorkerId(workerName), data.SessionId(sessionId), addrPort, kcommon.GetWallTimeMs(), int32(capacity), int32(memorySize), nil, data.ST_MEMORY)
	slog.InfoContext(ctx, "Worker info created",
		slog.String("event", "WorkerInfo"),
		slog.Any("workerId", workerInfo.WorkerId),
		slog.Any("sessionId", workerInfo.SessionId),
		slog.Any("addressPort", workerInfo.AddressPort),
	)

	// 创建主路由
	app := biz.NewApp()
	cougarApp := biz.NewMyCougarApp()
	cougar := cougar.NewCougarBuilder().WithCougarApp(cougarApp).WithWorkerInfo(workerInfo).Build(ctx)
	h := handler.NewHandler(app, cougarApp)
	mainMux := http.NewServeMux()
	h.RegisterRoutes(mainMux)
	shuttingDown := false
	go func() {
		reason := cougar.WaitOnExit()
		if shuttingDown {
			slog.InfoContext(ctx, "Cougar exited gracefully",
				slog.String("event", "CougarExit"),
				slog.String("reason", reason),
			)
		} else {
			slog.ErrorContext(ctx, "Cougar exited",
				slog.String("event", "CougarExit"),
				slog.String("reason", reason),
			)
			os.Exit(1)
		}
	}()

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
	slog.InfoContext(ctx, "Server ports configuration",
		slog.String("event", "ServerConfig"),
		slog.Int("api_port", apiPort),
		slog.Int("metrics_port", metricsPort),
	)

	// 优雅关闭
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		slog.InfoContext(ctx, "Shutting down servers...",
			slog.String("event", "ServerShutdown"))
		// shutdown request
		chShutdownPermitted := cougar.RequestShutdown()
		slog.InfoContext(ctx, "Waiting for shutdown permit",
			slog.String("event", "ServerShutdown"))
		start := kcommon.GetWallTimeMs()
		<-chShutdownPermitted
		elapsed := kcommon.GetWallTimeMs() - start
		slog.InfoContext(ctx, "Shutdown permit received",
			slog.String("event", "ServerShutdown"),
			slog.Int64("elapsedMs", elapsed))
		cougar.StopLease(ctx)

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
