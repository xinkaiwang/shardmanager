package handler

import (
	"encoding/json"
	"net/http"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kmetrics"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/api"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/biz"
)

// Handler 处理 HTTP 请求
type Handler struct {
	app *biz.App
}

// NewHandler 创建一个新的 Handler 实例
func NewHandler(app *biz.App) *Handler {
	return &Handler{app: app}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// 包装所有处理器以添加错误处理中间件
	mux.Handle("/api/ping", ErrorHandlingMiddleware(http.HandlerFunc(h.PingHandler)))
	mux.Handle("/api/get_state", ErrorHandlingMiddleware(http.HandlerFunc(h.GetStateHandler)))
	mux.Handle("/api/snapshot_current", ErrorHandlingMiddleware(http.HandlerFunc(h.GetCurrentSnapshotHandler)))
	mux.Handle("/api/snapshot_future", ErrorHandlingMiddleware(http.HandlerFunc(h.GetFutureSnapshotHandler)))
}

// PingHandler 处理 /api/ping 请求
func (h *Handler) PingHandler(w http.ResponseWriter, r *http.Request) {
	// 设置响应头
	w.Header().Set("Content-Type", "application/json")

	// 只允许 GET 方法
	if r.Method != http.MethodGet {
		panic(kerror.Create("MethodNotAllowed", "only GET method is allowed").
			WithErrorCode(kerror.EC_INVALID_PARAMETER))
	}

	// 记录请求信息
	klogging.Verbose(r.Context()).
		Log("PingRequest", "received ping request")

	// 处理请求
	var resp string
	kmetrics.InstrumentSummaryRunVoid(r.Context(), "biz.Ping", func() {
		resp = h.app.Ping(r.Context())
	}, "")

	// 记录响应信息
	klogging.Info(r.Context()).
		With("status", 200).
		With("version", resp).
		Log("PingResponse", "sending ping response")

	// 返回响应
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		panic(kerror.Create("EncodingError", "failed to encode response").
			WithErrorCode(kerror.EC_INTERNAL_ERROR).
			With("error", err.Error()))
	}
}

// GetStatusHandler 处理 /api/get_state 请求
func (h *Handler) GetStateHandler(w http.ResponseWriter, r *http.Request) {
	// 设置响应头
	w.Header().Set("Content-Type", "application/json")

	// 只允许 GET 方法
	if r.Method != http.MethodGet {
		panic(kerror.Create("MethodNotAllowed", "only GET method is allowed").
			WithErrorCode(kerror.EC_INVALID_PARAMETER))
	}

	// 记录请求信息
	klogging.Verbose(r.Context()).
		Log("GetStatusRequest", "received get status request")

	// 处理请求
	var resp *api.GetStateResponse
	kmetrics.InstrumentSummaryRunVoid(r.Context(), "biz.GetStatus", func() {
		resp = h.app.GetStatus(r.Context(), &api.GetStateRequest{})
	}, "")

	// 记录响应信息
	klogging.Info(r.Context()).
		With("worker_count", len(resp.Workers)).
		Log("GetStatusResponse", "sending get status response")

	// 返回响应
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		panic(kerror.Create("EncodingError", "failed to encode response").
			WithErrorCode(kerror.EC_INTERNAL_ERROR).
			With("error", err.Error()))
	}
}

func (h *Handler) GetCurrentSnapshotHandler(w http.ResponseWriter, r *http.Request) {
	// 设置响应头
	w.Header().Set("Content-Type", "application/json")

	// 只允许 GET 方法
	if r.Method != http.MethodGet {
		panic(kerror.Create("MethodNotAllowed", "only GET method is allowed").
			WithErrorCode(kerror.EC_INVALID_PARAMETER))
	}

	// 处理请求
	snapshot := h.app.GetCurrentSnapshot(r.Context())

	// 返回响应
	if err := json.NewEncoder(w).Encode(snapshot); err != nil {
		panic(kerror.Create("EncodingError", "failed to encode response").
			WithErrorCode(kerror.EC_INTERNAL_ERROR).
			With("error", err.Error()))
	}
}
func (h *Handler) GetFutureSnapshotHandler(w http.ResponseWriter, r *http.Request) {
	// 设置响应头
	w.Header().Set("Content-Type", "application/json")

	// 只允许 GET 方法
	if r.Method != http.MethodGet {
		panic(kerror.Create("MethodNotAllowed", "only GET method is allowed").
			WithErrorCode(kerror.EC_INVALID_PARAMETER))
	}

	// 处理请求
	snapshot := h.app.GetFutureSnapshot(r.Context())

	// 返回响应
	if err := json.NewEncoder(w).Encode(snapshot); err != nil {
		panic(kerror.Create("EncodingError", "failed to encode response").
			WithErrorCode(kerror.EC_INTERNAL_ERROR).
			With("error", err.Error()))
	}
}
