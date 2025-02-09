package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/xinkai/cursor/shardmanager/services/hellosvc/internal"
)

func main() {
	// 创建服务实例
	svc := internal.NewService()
	handler := internal.NewHandler(svc)

	// 创建路由
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// 创建 HTTP 服务器
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// 优雅关闭
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown error: %v\n", err)
		}
	}()

	// 启动服务器
	log.Printf("Server starting on %s\n", server.Addr)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v\n", err)
	}
	log.Println("Server stopped")
}
