package internal

import (
	"encoding/json"
	"net/http"

	"github.com/xinkai/cursor/shardmanager/services/hellosvc/api"
)

// Handler 处理 HTTP 请求
type Handler struct {
	svc *Service
}

// NewHandler 创建一个新的 Handler 实例
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// HelloHandler 处理 /hello 请求
func (h *Handler) HelloHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req api.HelloRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	resp := h.svc.SayHello(&req)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/hello", h.HelloHandler)
}
