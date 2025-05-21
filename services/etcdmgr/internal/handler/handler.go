package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

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
	// API 路由
	mux.Handle("/api/status", ErrorHandlingMiddleware(http.HandlerFunc(h.StatusHandler)))
	mux.Handle("/api/ping", ErrorHandlingMiddleware(http.HandlerFunc(h.PingHandler)))
	mux.Handle("/api/list_keys", ErrorHandlingMiddleware(http.HandlerFunc(h.ListKeysHandler)))
	mux.Handle("/api/get_key", ErrorHandlingMiddleware(http.HandlerFunc(h.GetKeyHandler)))
	mux.Handle("/api/set_key", ErrorHandlingMiddleware(http.HandlerFunc(h.SetKeyHandler)))
	mux.Handle("/api/delete_key", ErrorHandlingMiddleware(http.HandlerFunc(h.DeleteKeyHandler)))

	// 静态文件服务 (SPA)
	// IMPORTANT: This handler should be registered LAST as it catches all non-API paths.
	staticFs := http.FileServer(http.Dir("web/dist"))
	mux.Handle("/", h.SPAHandler("web/dist", staticFs))
}

// SPAHandler creates a handler that serves static files and handles SPA routing.
func (h *Handler) SPAHandler(staticPath string, fileSystem http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Let API requests fall through to previous handlers (ServeMux handles this naturally)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			// This case should ideally not be reached if API handlers are registered first,
			// but as a safeguard, return 404.
			http.NotFound(w, r)
			return
		}

		// Construct the path for the requested file.
		filePath := path.Join(staticPath, r.URL.Path)

		// Check if the requested file exists.
		_, err := os.Stat(filePath)
		if os.IsNotExist(err) {
			// File does not exist, serve index.html for SPA routing.
			klogging.Debug(r.Context()).With("path", filePath).Log("SPAHandler", "File not found, serving index.html")
			http.ServeFile(w, r, path.Join(staticPath, "index.html"))
			return
		}
		if err != nil {
			// Other error (e.g., permission denied)
			klogging.Error(r.Context()).WithError(err).With("path", filePath).Log("SPAHandler", "Error checking file existence")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// File exists, serve it using the provided fileSystem handler.
		klogging.Debug(r.Context()).With("path", filePath).Log("SPAHandler", "Serving static file")
		fileSystem.ServeHTTP(w, r)
	})
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
	klogging.Verbose(r.Context()).Log("PingRequest", "received ping request")

	// 处理请求
	var resp api.PingResponse
	kmetrics.InstrumentSummaryRunVoid(r.Context(), "biz.Ping", func() {
		resp = h.app.Ping(r.Context())
	}, "")

	// 记录响应信息
	klogging.Info(r.Context()).
		With("status", resp.Status).
		With("version", resp.Version).
		Log("PingResponse", "sending ping response")

	// 返回响应
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		panic(kerror.Create("EncodingError", "failed to encode response").
			WithErrorCode(kerror.EC_INTERNAL_ERROR).
			With("error", err.Error()))
	}
}

// ListKeysHandler 处理 /api/list_keys 请求，用于列出键值对
func (h *Handler) ListKeysHandler(w http.ResponseWriter, r *http.Request) {
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

	// 获取分页参数
	limitStr := r.URL.Query().Get("limit")
	limit := 0 // 默认不限制
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			panic(kerror.Create("InvalidParameter", "invalid limit parameter").
				WithErrorCode(kerror.EC_INVALID_PARAMETER).
				With("error", err.Error()))
		}
	}

	// 设置一个合理的默认值
	if limit <= 0 {
		limit = 20 // 默认每页 20 条
	}

	// 设置一个合理的最大限制，防止请求太大
	if limit > 1000 {
		limit = 1000
	}

	// 获取分页 token
	nextToken := r.URL.Query().Get("nextToken")

	// 记录请求信息
	klogging.Verbose(r.Context()).
		With("prefix", prefix).
		With("limit", limit).
		With("hasNextToken", nextToken != "").
		Log("ListKeysRequest", "received list keys request")

	// 处理请求
	var resp *api.EtcdKeysResponse
	kmetrics.InstrumentSummaryRunVoid(r.Context(), "biz.ListKeys", func() {
		var err error
		resp, err = h.app.ListKeys(r.Context(), prefix, limit, nextToken)
		if err != nil {
			panic(err)
		}
	}, "")

	// 记录响应信息
	klogging.Info(r.Context()).
		With("prefix", prefix).
		With("count", len(resp.Keys)).
		With("hasMore", resp.NextToken != "").
		Log("ListKeysResponse", "sending list keys response")

	// 返回响应
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		panic(kerror.Create("EncodingError", "failed to encode response").
			WithErrorCode(kerror.EC_INTERNAL_ERROR).
			With("error", err.Error()))
	}
}

// GetKeyHandler 处理 /api/get_key 请求
func (h *Handler) GetKeyHandler(w http.ResponseWriter, r *http.Request) {
	// 设置响应头
	w.Header().Set("Content-Type", "application/json")

	// 只允许 GET 方法
	if r.Method != http.MethodGet {
		panic(kerror.Create("MethodNotAllowed", "only GET method is allowed").
			WithErrorCode(kerror.EC_INVALID_PARAMETER))
	}

	// 从查询参数获取键名
	key := r.URL.Query().Get("key")
	if key == "" {
		panic(kerror.Create("InvalidKey", "key is required").
			WithErrorCode(kerror.EC_INVALID_PARAMETER))
	}

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

// SetKeyHandler 处理 /api/set_key 请求
func (h *Handler) SetKeyHandler(w http.ResponseWriter, r *http.Request) {
	// 设置响应头
	w.Header().Set("Content-Type", "application/json")

	// 只允许 POST 方法
	if r.Method != http.MethodPost {
		panic(kerror.Create("MethodNotAllowed", "only POST method is allowed").
			WithErrorCode(kerror.EC_INVALID_PARAMETER))
	}

	// 从查询参数获取键名
	key := r.URL.Query().Get("key")
	if key == "" {
		panic(kerror.Create("InvalidKey", "key is required").
			WithErrorCode(kerror.EC_INVALID_PARAMETER))
	}

	var req api.EtcdKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		panic(kerror.Create("InvalidRequest", "invalid request format").
			WithErrorCode(kerror.EC_INVALID_PARAMETER).
			With("error", err.Error()))
	}

	klogging.Verbose(r.Context()).
		With("key", key).
		Log("SetKeyRequest", "received set key request")

	kmetrics.InstrumentSummaryRunVoid(r.Context(), "biz.PutKey", func() {
		if err := h.app.PutKey(r.Context(), key, req.Value); err != nil {
			panic(err)
		}
	}, "")

	klogging.Info(r.Context()).
		With("key", key).
		Log("SetKeyResponse", "key updated successfully")

	w.WriteHeader(http.StatusOK)
}

// DeleteKeyHandler 处理 /api/delete_key 请求
func (h *Handler) DeleteKeyHandler(w http.ResponseWriter, r *http.Request) {
	// 设置响应头
	w.Header().Set("Content-Type", "application/json")

	// 只允许 POST 方法
	if r.Method != http.MethodPost {
		panic(kerror.Create("MethodNotAllowed", "only POST method is allowed").
			WithErrorCode(kerror.EC_INVALID_PARAMETER))
	}

	// 从查询参数获取键名
	key := r.URL.Query().Get("key")
	if key == "" {
		panic(kerror.Create("InvalidKey", "key is required").
			WithErrorCode(kerror.EC_INVALID_PARAMETER))
	}

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
