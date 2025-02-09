package internal

import (
	"fmt"
	"time"

	"github.com/xinkai/cursor/shardmanager/services/hellosvc/api"
)

// Service 实现了 hello 服务的业务逻辑
type Service struct{}

// NewService 创建一个新的 Service 实例
func NewService() *Service {
	return &Service{}
}

// SayHello 处理 hello 请求
func (s *Service) SayHello(req *api.HelloRequest) *api.HelloResponse {
	message := fmt.Sprintf("Hello, %s!", req.Name)
	if req.Name == "" {
		message = "Hello, World!"
	}

	return &api.HelloResponse{
		Message: message,
		Time:    time.Now().Format(time.RFC3339),
	}
}
