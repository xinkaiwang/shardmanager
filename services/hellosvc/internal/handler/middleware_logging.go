package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
)

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// 处理错误并记录
				var ke *kerror.Kerror
				switch v := err.(type) {
				case *kerror.Kerror:
					// 如果已经是 kerror，直接使用
					ke = v
					slog.ErrorContext(r.Context(), "panic recovered in middleware",
						slog.String("event", "PanicRecovered"),
						slog.Any("error", ke))
				case error:
					// 如果是普通 error，包装成 kerror
					ke = kerror.Create("InternalServerError", v.Error()).
						WithErrorCode(kerror.EC_UNKNOWN)
					slog.ErrorContext(r.Context(), "panic recovered in middleware",
						slog.String("event", "PanicRecovered"),
						slog.Any("error", ke))
				default:
					// 其他类型（如 string 或其他值），创建新的 kerror
					ke = kerror.Create("UnknownPanic", "unexpected panic with non-error value").
						WithErrorCode(kerror.EC_UNKNOWN).
						With("panic_value", v)
					slog.ErrorContext(r.Context(), "panic recovered in middleware",
						slog.String("event", "PanicRecovered"),
						slog.Any("panic_value", v))
				}

				// 使用 ErrorCode 的 ToHttpErrorCode 方法设置状态码
				w.WriteHeader(ke.ErrorCode.ToHttpErrorCode())

				// 返回错误响应
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error": ke.Type,
					"msg":   ke.Msg,
					"code":  ke.ErrorCode,
				})
			}
		}()

		// 调用下一个处理器
		next.ServeHTTP(w, r)
	})
}
