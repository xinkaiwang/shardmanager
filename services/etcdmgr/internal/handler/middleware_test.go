package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
)

// withSilentLog 临时禁用日志输出
func withSilentLog(t *testing.T, fn func()) {
	// 保存当前的 logger
	oldLogger := klogging.GetDefaultLogger()
	// 设置 null logger
	klogging.SetDefaultLogger(klogging.NewNullLogger())
	// 确保在函数结束时恢复原来的 logger
	defer klogging.SetDefaultLogger(oldLogger)
	// 执行测试函数
	fn()
}

func TestErrorHandlingMiddleware(t *testing.T) {
	tests := []struct {
		name          string
		handler       func(w http.ResponseWriter, r *http.Request)
		wantStatus    int
		wantErrorType string
		wantErrorMsg  string
	}{
		{
			name: "已有的 kerror",
			handler: func(w http.ResponseWriter, r *http.Request) {
				panic(kerror.Create("InvalidParam", "参数无效").WithErrorCode(kerror.EC_INVALID_PARAMETER))
			},
			wantStatus:    http.StatusBadRequest,
			wantErrorType: "InvalidParam",
			wantErrorMsg:  "参数无效",
		},
		{
			name: "普通 error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				panic(kerror.Create("InternalError", "内部错误").WithErrorCode(kerror.EC_INTERNAL_ERROR))
			},
			wantStatus:    http.StatusServiceUnavailable,
			wantErrorType: "InternalError",
			wantErrorMsg:  "内部错误",
		},
		{
			name: "字符串 panic",
			handler: func(w http.ResponseWriter, r *http.Request) {
				panic("未知错误")
			},
			wantStatus:    http.StatusInternalServerError,
			wantErrorType: "UnknownError",
			wantErrorMsg:  "未知错误",
		},
		{
			name: "正常请求",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ok"))
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withSilentLog(t, func() {
				// 创建一个测试请求
				req := httptest.NewRequest("GET", "/test", nil)
				// 创建一个响应记录器
				w := httptest.NewRecorder()

				// 创建处理器
				handler := http.HandlerFunc(tt.handler)
				// 包装处理器
				wrapped := ErrorHandlingMiddleware(handler)
				// 执行请求
				wrapped.ServeHTTP(w, req)

				// 验证状态码
				if w.Code != tt.wantStatus {
					t.Errorf("状态码错误：got %v, want %v", w.Code, tt.wantStatus)
				}

				// 如果期望有错误信息，验证响应内容
				if tt.wantErrorType != "" {
					resp := w.Body.String()
					if resp == "" {
						t.Error("响应内容为空")
					}
				}
			})
		})
	}
}

func TestErrorCodeMapping(t *testing.T) {
	tests := []struct {
		name       string
		errorCode  kerror.ErrorCode
		wantStatus int
	}{
		{
			name:       "EC_INVALID_PARAMETER -> 400",
			errorCode:  kerror.EC_INVALID_PARAMETER,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "EC_NOT_FOUND -> 404",
			errorCode:  kerror.EC_NOT_FOUND,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "EC_CONFLICT -> 409",
			errorCode:  kerror.EC_CONFLICT,
			wantStatus: http.StatusConflict,
		},
		{
			name:       "EC_INTERNAL_ERROR -> 503",
			errorCode:  kerror.EC_INTERNAL_ERROR,
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name:       "EC_RETRYABLE -> 429",
			errorCode:  kerror.EC_RETRYABLE,
			wantStatus: http.StatusTooManyRequests,
		},
		{
			name:       "EC_UNKNOWN -> 500",
			errorCode:  kerror.EC_UNKNOWN,
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withSilentLog(t, func() {
				// 创建一个测试请求
				req := httptest.NewRequest("GET", "/test", nil)
				// 创建一个响应记录器
				w := httptest.NewRecorder()

				// 创建处理器
				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					panic(kerror.Create("TestError", "测试错误").WithErrorCode(tt.errorCode))
				})
				// 包装处理器
				wrapped := ErrorHandlingMiddleware(handler)
				// 执行请求
				wrapped.ServeHTTP(w, req)

				// 验证状态码
				if w.Code != tt.wantStatus {
					t.Errorf("状态码错误：got %v, want %v", w.Code, tt.wantStatus)
				}
			})
		})
	}
}

// 测试 Content-Type 设置
func TestErrorHandlingMiddleware_ContentType(t *testing.T) {
	withSilentLog(t, func() {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic(kerror.Create("TestError", "test error").
				WithErrorCode(kerror.EC_INVALID_PARAMETER))
		})

		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		middleware := ErrorHandlingMiddleware(handler)
		middleware.ServeHTTP(rr, req)

		contentType := rr.Header().Get("Content-Type")
		expectedContentType := "application/json"
		if contentType != expectedContentType {
			t.Errorf("Content-Type 错误：期望 %s，得到 %s", expectedContentType, contentType)
		}
	})
}
