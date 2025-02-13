package handler

import (
	"encoding/json"
	"net/http"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kmetrics"
	"github.com/xinkaiwang/shardmanager/services/etcdmgr/api"
	"github.com/xinkaiwang/shardmanager/services/etcdmgr/internal/biz"
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
	mux.Handle("/api/status", ErrorHandlingMiddleware(http.HandlerFunc(h.StatusHandler)))
	mux.Handle("/api/keys", ErrorHandlingMiddleware(http.HandlerFunc(h.KeysHandler)))
	mux.Handle("/api/key/", ErrorHandlingMiddleware(http.HandlerFunc(h.KeyHandler)))
}

// StatusHandler 处理 /api/status 请求
func (h *Handler) StatusHandler(w http.ResponseWriter, r *http.Request) {
	// 设置响应头
	w.Header().Set("Content-Type", "application/json")

	// 只允许 GET 方法
	if r.Method != http.MethodGet {
		panic(kerror.Create("MethodNotAllowed", "only GET method is allowed").
			WithErrorCode(kerror.EC_INVALID_PARAMETER))
	}

	// 记录请求信息
	klogging.Verbose(r.Context()).Log("StatusRequest", "received status request")

	// 处理请求
	var resp api.StatusResponse
	kmetrics.InstrumentSummaryRunVoid(r.Context(), "biz.Status", func() {
		resp = h.app.Status(r.Context())
	}, "")

	// 记录响应信息
	klogging.Info(r.Context()).
		With("status", resp.Status).
		With("version", resp.Version).
		Log("StatusResponse", "sending status response")

	// 返回响应
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		panic(kerror.Create("EncodingError", "failed to encode response").
			WithErrorCode(kerror.EC_INTERNAL_ERROR).
			With("error", err.Error()))
	}
}

// KeysHandler 处理 /api/keys 请求，用于列出键值对
func (h *Handler) KeysHandler(w http.ResponseWriter, r *http.Request) {
	// 设置响应头
	w.Header().Set("Content-Type", "application/json")

	// 只允许 GET 方法
	if r.Method != http.MethodGet {
		panic(kerror.Create("MethodNotAllowed", "only GET method is allowed").
			WithErrorCode(kerror.EC_INVALID_PARAMETER))
	}

	// 获取查询参数
	prefix := r.URL.Query().Get("prefix")
	if prefix == "" {
		prefix = "" // 默认列出所有键
	}

	// 记录请求信息
	klogging.Verbose(r.Context()).
		With("prefix", prefix).
		Log("ListKeysRequest", "received list keys request")

	// 处理请求
	var resp *api.EtcdKeysResponse
	kmetrics.InstrumentSummaryRunVoid(r.Context(), "biz.ListKeys", func() {
		var err error
		resp, err = h.app.ListKeys(r.Context(), prefix)
		if err != nil {
			panic(err)
		}
	}, "")

	// 记录响应信息
	klogging.Info(r.Context()).
		With("prefix", prefix).
		With("count", len(resp.Keys)).
		Log("ListKeysResponse", "sending list keys response")

	// 返回响应
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		panic(kerror.Create("EncodingError", "failed to encode response").
			WithErrorCode(kerror.EC_INTERNAL_ERROR).
			With("error", err.Error()))
	}
}

// KeyHandler 处理 /api/key/{key} 请求，用于获取、设置或删除单个键值对
func (h *Handler) KeyHandler(w http.ResponseWriter, r *http.Request) {
	// 设置响应头
	w.Header().Set("Content-Type", "application/json")

	// 从路径中提取键名
	key := r.URL.Path[len("/api/key/"):]
	if key == "" {
		panic(kerror.Create("InvalidKey", "key is required").
			WithErrorCode(kerror.EC_INVALID_PARAMETER))
	}

	switch r.Method {
	case http.MethodGet:
		h.handleGetKey(w, r, key)
	case http.MethodPut:
		h.handlePutKey(w, r, key)
	case http.MethodDelete:
		h.handleDeleteKey(w, r, key)
	default:
		panic(kerror.Create("MethodNotAllowed", "method not allowed").
			WithErrorCode(kerror.EC_INVALID_PARAMETER).
			With("method", r.Method))
	}
}

// handleGetKey 处理获取键值的请求
func (h *Handler) handleGetKey(w http.ResponseWriter, r *http.Request, key string) {
	klogging.Verbose(r.Context()).
		With("key", key).
		Log("GetKeyRequest", "received get key request")

	var resp *api.EtcdKeyResponse
	kmetrics.InstrumentSummaryRunVoid(r.Context(), "biz.GetKey", func() {
		var err error
		resp, err = h.app.GetKey(r.Context(), key)
		if err != nil {
			panic(err)
		}
	}, "")

	klogging.Info(r.Context()).
		With("key", key).
		With("found", resp != nil).
		Log("GetKeyResponse", "sending get key response")

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		panic(kerror.Create("EncodingError", "failed to encode response").
			WithErrorCode(kerror.EC_INTERNAL_ERROR).
			With("error", err.Error()))
	}
}

// handlePutKey 处理设置键值的请求
func (h *Handler) handlePutKey(w http.ResponseWriter, r *http.Request, key string) {
	var req api.EtcdKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		panic(kerror.Create("InvalidRequest", "invalid request format").
			WithErrorCode(kerror.EC_INVALID_PARAMETER).
			With("error", err.Error()))
	}

	klogging.Verbose(r.Context()).
		With("key", key).
		Log("PutKeyRequest", "received put key request")

	kmetrics.InstrumentSummaryRunVoid(r.Context(), "biz.PutKey", func() {
		if err := h.app.PutKey(r.Context(), key, req.Value); err != nil {
			panic(err)
		}
	}, "")

	klogging.Info(r.Context()).
		With("key", key).
		Log("PutKeyResponse", "key updated successfully")

	w.WriteHeader(http.StatusOK)
}

// handleDeleteKey 处理删除键值的请求
func (h *Handler) handleDeleteKey(w http.ResponseWriter, r *http.Request, key string) {
	klogging.Verbose(r.Context()).
		With("key", key).
		Log("DeleteKeyRequest", "received delete key request")

	kmetrics.InstrumentSummaryRunVoid(r.Context(), "biz.DeleteKey", func() {
		if err := h.app.DeleteKey(r.Context(), key); err != nil {
			panic(err)
		}
	}, "")

	klogging.Info(r.Context()).
		With("key", key).
		Log("DeleteKeyResponse", "key deleted successfully")

	w.WriteHeader(http.StatusOK)
}
