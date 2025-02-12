package biz

import (
	"context"
	"net/http"
	"time"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kmetrics"
)

var (
	// 指标定义
	RequestLatencyMetric = kmetrics.CreateKmetric(
		context.Background(),
		"blitz_request_latency_ms",
		"Request latency in milliseconds",
		[]string{"status_code", "error"},
	)
)

// LoadTestResult 表示单次负载测试的结果
type LoadTestResult struct {
	StatusCode int     `json:"status_code"`
	LatencyMs  float64 `json:"latency_ms"`
	Error      string  `json:"error,omitempty"`
}

type App struct {
	client *http.Client
}

func NewApp() *App {
	return &App{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// RunLoadTest 执行单次负载测试请求
func (app *App) RunLoadTest(ctx context.Context, targetURL string) LoadTestResult {
	startTime := time.Now()

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		latencyMs := time.Since(startTime).Milliseconds()
		RequestLatencyMetric.GetTimeSequence(ctx, "0", err.Error()).Add(latencyMs)
		return LoadTestResult{
			StatusCode: 0,
			LatencyMs:  float64(latencyMs),
			Error:      err.Error(),
		}
	}

	// 发送请求
	resp, err := app.client.Do(req)
	latencyMs := time.Since(startTime).Milliseconds()

	if err != nil {
		RequestLatencyMetric.GetTimeSequence(ctx, "0", err.Error()).Add(latencyMs)
		return LoadTestResult{
			StatusCode: 0,
			LatencyMs:  float64(latencyMs),
			Error:      err.Error(),
		}
	}
	defer resp.Body.Close()

	// 记录指标
	statusCode := resp.StatusCode
	var errorStr string
	if statusCode >= 400 {
		errorStr = resp.Status
	}
	RequestLatencyMetric.GetTimeSequence(ctx, string(rune(statusCode)), errorStr).Add(latencyMs)

	return LoadTestResult{
		StatusCode: statusCode,
		LatencyMs:  float64(latencyMs),
		Error:      errorStr,
	}
}
