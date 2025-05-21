package main

import (
	"context"
	"fmt"
	"log"
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
	klogging.Info(ctx).With("workerId", workerInfo.WorkerId).With("sessionId", workerInfo.SessionId).With("addressPort", workerInfo.AddressPort).Log("WorkerInfo", "Worker info created")

	// 创建主路由
	app := biz.NewApp()
	cougarApp := biz.NewMyCougarApp()
	cougar := cougar.NewCougarBuilder().WithCougarApp(cougarApp).WithWorkerInfo(workerInfo).Build(ctx)
	h := handler.NewHandler(app, cougarApp)
	mainMux := http.NewServeMux()
	h.RegisterRoutes(mainMux)
	go func() {
		reason := cougar.WaitOnExit()
		klogging.Fatal(ctx).With("reason", reason).Log("CougarExit", "Cougar exited")
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
		// shutdown request
		chDone := cougar.RequestShutdown()
		klogging.Info(ctx).Log("ServerShutdown", "Waiting for shutdown permit")
		start := kcommon.GetWallTimeMs()
		<-chDone
		elapsed := kcommon.GetWallTimeMs() - start
		klogging.Info(ctx).With("elapsedMs", elapsed).Log("ServerShutdown", "Shutdown permit received")

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
