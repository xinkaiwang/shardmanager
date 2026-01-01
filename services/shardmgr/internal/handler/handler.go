package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kmetrics"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/api"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/biz"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
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
	mux.Handle("/api/get_shard_plan", ErrorHandlingMiddleware(http.HandlerFunc(h.GetShardPlanHandler)))
	mux.Handle("/api/set_shard_plan", ErrorHandlingMiddleware(http.HandlerFunc(h.SetShardPlanHandler)))
	mux.Handle("/api/get_service_config", ErrorHandlingMiddleware(http.HandlerFunc(h.GetServiceConfigHandler)))
	mux.Handle("/api/set_service_config", ErrorHandlingMiddleware(http.HandlerFunc(h.SetServiceConfigHandler)))
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

// GetShardPlanHandler 处理 /api/get_shard_plan 请求
func (h *Handler) GetShardPlanHandler(w http.ResponseWriter, r *http.Request) {
	// 设置响应头
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	// 只允许 GET 方法
	if r.Method != http.MethodGet {
		panic(kerror.Create("MethodNotAllowed", "only GET method is allowed").
			WithErrorCode(kerror.EC_INVALID_PARAMETER))
	}

	// 记录请求信息
	klogging.Verbose(r.Context()).
		Log("GetShardPlanRequest", "received get shard plan request")

	// 处理请求
	var shardPlan string
	kmetrics.InstrumentSummaryRunVoid(r.Context(), "biz.GetShardPlan", func() {
		shardPlan = h.app.GetShardPlan(r.Context())
	}, "")

	// 记录响应信息
	klogging.Info(r.Context()).
		With("length", len(shardPlan)).
		Log("GetShardPlanResponse", "sending get shard plan response")

	// 返回纯文本响应
	w.Write([]byte(shardPlan))
}

// SetShardPlanHandler 处理 /api/set_shard_plan 请求
func (h *Handler) SetShardPlanHandler(w http.ResponseWriter, r *http.Request) {
	// 设置响应头
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	// 只允许 POST 方法
	if r.Method != http.MethodPost {
		panic(kerror.Create("MethodNotAllowed", "only POST method is allowed").
			WithErrorCode(kerror.EC_INVALID_PARAMETER))
	}

	// 读取请求体
	body, err := io.ReadAll(r.Body)
	if err != nil {
		panic(kerror.Create("ReadBodyError", "failed to read request body").
			WithErrorCode(kerror.EC_INVALID_PARAMETER).
			With("error", err.Error()))
	}
	defer r.Body.Close()

	shardPlan := string(body)

	// 记录请求信息
	klogging.Info(r.Context()).
		With("length", len(shardPlan)).
		Log("SetShardPlanRequest", "received set shard plan request")

	// 处理请求
	kmetrics.InstrumentSummaryRunVoid(r.Context(), "biz.SetShardPlan", func() {
		h.app.WriteShardPlan(r.Context(), shardPlan)
	}, "")

	// 记录响应信息
	klogging.Info(r.Context()).
		Log("SetShardPlanResponse", "shard plan updated successfully")

	// 返回成功响应
	w.Write([]byte("OK\n"))
}

// GetServiceConfigHandler 处理 /api/get_service_config 请求
func (h *Handler) GetServiceConfigHandler(w http.ResponseWriter, r *http.Request) {
	// 设置响应头
	w.Header().Set("Content-Type", "application/json")

	// 只允许 GET 方法
	if r.Method != http.MethodGet {
		panic(kerror.Create("MethodNotAllowed", "only GET method is allowed").
			WithErrorCode(kerror.EC_INVALID_PARAMETER))
	}

	// 记录请求信息
	klogging.Verbose(r.Context()).
		Log("GetServiceConfigRequest", "received get service config request")

	// 处理请求
	var configJson *smgjson.ServiceConfigJson
	kmetrics.InstrumentSummaryRunVoid(r.Context(), "biz.GetServiceConfig", func() {
		configJson = h.app.GetServiceConfig(r.Context())
	}, "")

	// 记录响应信息
	klogging.Info(r.Context()).
		Log("GetServiceConfigResponse", "sending get service config response")

	// 返回 JSON 响应
	data, err := json.Marshal(configJson)
	if err != nil {
		panic(kerror.Create("EncodingError", "failed to encode response").
			WithErrorCode(kerror.EC_INTERNAL_ERROR).
			With("error", err.Error()))
	}
	w.Write(data)
}

// SetServiceConfigHandler 处理 /api/set_service_config 请求
func (h *Handler) SetServiceConfigHandler(w http.ResponseWriter, r *http.Request) {
	// 设置响应头
	w.Header().Set("Content-Type", "application/json")

	// 只允许 POST 方法
	if r.Method != http.MethodPost {
		panic(kerror.Create("MethodNotAllowed", "only POST method is allowed").
			WithErrorCode(kerror.EC_INVALID_PARAMETER))
	}

	// 读取请求体
	body, err := io.ReadAll(r.Body)
	if err != nil {
		panic(kerror.Create("ReadBodyError", "failed to read request body").
			WithErrorCode(kerror.EC_INVALID_PARAMETER).
			With("error", err.Error()))
	}
	defer r.Body.Close()

	// 解析 JSON
	var partialConfig smgjson.ServiceConfigJson
	err = json.Unmarshal(body, &partialConfig)
	if err != nil {
		panic(kerror.Create("ParseError", "failed to parse request body").
			WithErrorCode(kerror.EC_INVALID_PARAMETER).
			With("error", err.Error()))
	}

	// 记录请求信息
	klogging.Info(r.Context()).
		With("config", string(body)).
		Log("SetServiceConfigRequest", "received set service config request")

	// 处理请求
	kmetrics.InstrumentSummaryRunVoid(r.Context(), "biz.SetServiceConfig", func() {
		h.app.SetServiceConfig(r.Context(), &partialConfig)
	}, "")

	// 记录响应信息
	klogging.Info(r.Context()).
		Log("SetServiceConfigResponse", "service config updated successfully")

	// 返回成功响应
	response := map[string]interface{}{
		"success": true,
		"message": "配置更新成功",
	}
	data, err := json.Marshal(response)
	if err != nil {
		panic(kerror.Create("EncodingError", "failed to encode response").
			WithErrorCode(kerror.EC_INTERNAL_ERROR).
			With("error", err.Error()))
	}
	w.Write(data)
}
