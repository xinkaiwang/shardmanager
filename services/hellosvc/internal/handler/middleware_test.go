package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
)

func TestErrorHandlingMiddleware(t *testing.T) {
	tests := []struct {
		name          string
		handler       http.HandlerFunc
		expectedCode  int
		expectedType  string
		expectedError string
	}{
		{
			name: "已有的 kerror",
			handler: func(w http.ResponseWriter, r *http.Request) {
				panic(kerror.Create("TestError", "test error message").
					WithErrorCode(kerror.EC_INVALID_PARAMETER))
			},
			expectedCode:  http.StatusBadRequest,
			expectedType:  "TestError",
			expectedError: "test error message",
		},
		{
			name: "普通 error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				panic(fmt.Errorf("some error"))
			},
			expectedCode:  http.StatusInternalServerError,
			expectedType:  "InternalServerError",
			expectedError: "an unexpected error occurred",
		},
		{
			name: "字符串 panic",
			handler: func(w http.ResponseWriter, r *http.Request) {
				panic("some panic message")
			},
			expectedCode:  http.StatusInternalServerError,
			expectedType:  "UnknownPanic",
			expectedError: "unexpected panic with non-error value",
		},
		{
			name: "正常请求",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			expectedCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建测试请求
			req := httptest.NewRequest("GET", "/test", nil)
			rr := httptest.NewRecorder()

			// 使用中间件包装处理器
			handler := ErrorHandlingMiddleware(http.HandlerFunc(tt.handler))
			handler.ServeHTTP(rr, req)

			// 检查状态码
			if rr.Code != tt.expectedCode {
				t.Errorf("状态码错误：期望 %d，得到 %d", tt.expectedCode, rr.Code)
			}

			// 如果期望有错误响应，解析并验证
			if tt.expectedType != "" {
				var response map[string]interface{}
				if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
					t.Fatalf("解析响应失败：%v", err)
				}

				// 验证错误类型
				if errorType, ok := response["error"].(string); !ok || errorType != tt.expectedType {
					t.Errorf("错误类型错误：期望 %s，得到 %v", tt.expectedType, response["error"])
				}

				// 验证错误消息
				if errorMsg, ok := response["msg"].(string); !ok || errorMsg != tt.expectedError {
					t.Errorf("错误消息错误：期望 %s，得到 %v", tt.expectedError, response["msg"])
				}
			}
		})
	}
}

// 测试 Content-Type 设置
func TestErrorHandlingMiddleware_ContentType(t *testing.T) {
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
}

// 测试错误码转换
func TestErrorHandlingMiddleware_ErrorCodeMapping(t *testing.T) {
	tests := []struct {
		name         string
		errorCode    kerror.ErrorCode
		expectedHTTP int
	}{
		{"无效参数", kerror.EC_INVALID_PARAMETER, http.StatusBadRequest},
		{"未找到", kerror.EC_NOT_FOUND, http.StatusNotFound},
		{"冲突", kerror.EC_CONFLICT, http.StatusConflict},
		{"内部错误", kerror.EC_INTERNAL_ERROR, http.StatusServiceUnavailable},
		{"可重试", kerror.EC_RETRYABLE, http.StatusTooManyRequests},
		{"未知错误", kerror.EC_UNKNOWN, http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				panic(kerror.Create("TestError", "test error").
					WithErrorCode(tt.errorCode))
			})

			req := httptest.NewRequest("GET", "/test", nil)
			rr := httptest.NewRecorder()

			middleware := ErrorHandlingMiddleware(handler)
			middleware.ServeHTTP(rr, req)

			if rr.Code != tt.expectedHTTP {
				t.Errorf("HTTP 状态码错误：期望 %d，得到 %d", tt.expectedHTTP, rr.Code)
			}
		})
	}
}
