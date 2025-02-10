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
	"github.com/xinkaiwang/shardmanager/libs/xklib/kmetrics"
	"github.com/xinkaiwang/shardmanager/services/hellosvc/internal/biz"
	"github.com/xinkaiwang/shardmanager/services/hellosvc/internal/handler"
	"go.opencensus.io/metric/metricproducer"
)

func main() {
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

		log.Println("Shutting down servers...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Main server shutdown error: %v\n", err)
		}
		if err := metricsServer.Shutdown(ctx); err != nil {
			log.Printf("Metrics server shutdown error: %v\n", err)
		}
	}()

	// 启动 metrics 服务器
	go func() {
		log.Printf("Metrics server starting on %s\n", metricsServer.Addr)
		if err := metricsServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("Metrics server error: %v\n", err)
		}
	}()

	// 启动主服务器
	log.Printf("Main server starting on %s\n", server.Addr)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Main server error: %v\n", err)
	}
	log.Println("Servers stopped")
}
