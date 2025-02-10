package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kmetrics"
	"github.com/xinkaiwang/shardmanager/services/hellosvc/api"
	"github.com/xinkaiwang/shardmanager/services/hellosvc/internal/biz"
)

// Handler 处理 HTTP 请求
type Handler struct {
	app *biz.App
}

// NewHandler 创建一个新的 Handler 实例
func NewHandler(app *biz.App) *Handler {
	return &Handler{app: app}
}

// HelloHandler 处理 /hello 请求
func (h *Handler) HelloHandler(w http.ResponseWriter, r *http.Request) {
	// 设置响应头
	w.Header().Set("Content-Type", "application/json")

	var req api.HelloRequest

	// 根据请求方法处理
	switch r.Method {
	case http.MethodGet:
		// 从查询参数获取名字
		name := r.URL.Query().Get("name")
		req = api.HelloRequest{Name: name}
	case http.MethodPost:
		// 从请求体解析 JSON
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			panic(kerror.Create("BadRequest", "invalid request format").
				WithErrorCode(kerror.EC_INVALID_PARAMETER).
				With("error", err.Error()))
		}
	default:
		panic(kerror.Create("MethodNotAllowed", "only GET and POST methods are allowed").
			WithErrorCode(kerror.EC_INVALID_PARAMETER))
	}

	// 记录请求信息
	klogging.Info(r.Context()).
		With("name", req.Name).
		Log("HelloRequest", "received hello request")

	// 处理请求
	var message string
	kmetrics.InstrumentSummaryRunVoid(r.Context(), "biz.Hello", func() {
		message = h.app.Hello(req.Name)
	}, "")

	resp := &api.HelloResponse{
		Message: message,
		Time:    time.Now().Format(time.RFC3339),
	}

	// 记录响应信息
	klogging.Info(r.Context()).
		With("message", resp.Message).
		With("time", resp.Time).
		Log("HelloResponse", "sending hello response")

	// 返回响应
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		panic(kerror.Create("EncodingError", "failed to encode response").
			WithErrorCode(kerror.EC_INTERNAL_ERROR).
			With("error", err.Error()))
	}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// 包装所有处理器以添加错误处理中间件
	mux.Handle("/api/ping", ErrorHandlingMiddleware(http.HandlerFunc(h.PingHandler)))
	mux.Handle("/api/hello", ErrorHandlingMiddleware(http.HandlerFunc(h.HelloHandler)))
	mux.Handle("/api/test_kerror", ErrorHandlingMiddleware(http.HandlerFunc(h.HelloKerrorHandler)))
	mux.Handle("/api/test_error", ErrorHandlingMiddleware(http.HandlerFunc(h.HelloErrorHandler)))
	mux.Handle("/api/test_panic", ErrorHandlingMiddleware(http.HandlerFunc(h.HelloPanicHandler)))
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
	klogging.Info(r.Context()).
		Log("PingRequest", "received ping request")

	// 处理请求
	resp := &api.PingResponse{
		Status:    "ok",
		Timestamp: time.Now().Format(time.RFC3339),
		Version:   h.app.GetVersion(),
	}

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

// HelloKerrorHandler 处理 /hello/kerror 请求，总是抛出 kerror
func (h *Handler) HelloKerrorHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "test"
	}
	kmetrics.InstrumentSummaryRunVoid(r.Context(), "biz.HelloWithKerror", func() {
		h.app.HelloWithKerror(name)
	}, "")
}

// HelloErrorHandler 处理 /hello/error 请求，总是抛出普通 error
func (h *Handler) HelloErrorHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "test"
	}
	kmetrics.InstrumentSummaryRunVoid(r.Context(), "biz.HelloWithError", func() {
		h.app.HelloWithError(name)
	}, "")
}

// HelloPanicHandler 处理 /hello/panic 请求，总是抛出非错误类型的 panic
func (h *Handler) HelloPanicHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "test"
	}
	kmetrics.InstrumentSummaryRunVoid(r.Context(), "biz.HelloWithPanic", func() {
		h.app.HelloWithPanic(name)
	}, "")
}
