package klogging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
)

func newJSONLogger(level slog.Level) (*slog.Logger, *bytes.Buffer, *Handler) {
	var buf bytes.Buffer
	h := NewHandler(&HandlerOptions{Level: level, Format: "json", Output: &buf})
	return slog.New(h), &buf, h
}

func lastLogLine(t *testing.T, buf *bytes.Buffer) map[string]interface{} {
	t.Helper()
	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	var m map[string]interface{}
	if err := json.Unmarshal(lines[len(lines)-1], &m); err != nil {
		t.Fatalf("bad json log line: %v: %s", err, buf.String())
	}
	return m
}

// KLOG-007 第 1 层：biz 层 CtxWithAttrs 一次，下游（dao）日志自动带字段
func TestCtxWithAttrs_AmbientField(t *testing.T) {
	logger, buf, _ := newJSONLogger(slog.LevelInfo)
	ctx := CtxWithAttrs(context.Background(), slog.String("orderId", "o-42"))

	// 模拟 dao 层：只拿到 ctx，不知道 orderId 的存在
	logger.InfoContext(ctx, "query executed", slog.String("event", "DaoQuery"))

	m := lastLogLine(t, buf)
	if m["orderId"] != "o-42" {
		t.Errorf("ambient field missing: %v", m)
	}
}

// 分层继承：子层新增字段与父层共存；兄弟分支互不污染（ctx 树形隔离）
func TestCtxWithAttrs_LayeringAndSiblingIsolation(t *testing.T) {
	logger, buf, _ := newJSONLogger(slog.LevelInfo)
	root := CtxWithAttrs(context.Background(), slog.String("workerId", "w-1"))
	childA := CtxWithAttrs(root, slog.String("shardId", "s-a"))
	childB := CtxWithAttrs(root, slog.String("shardId", "s-b"))

	logger.InfoContext(childA, "a")
	m := lastLogLine(t, buf)
	if m["workerId"] != "w-1" || m["shardId"] != "s-a" {
		t.Errorf("childA layers wrong: %v", m)
	}

	logger.InfoContext(childB, "b")
	m = lastLogLine(t, buf)
	if m["shardId"] != "s-b" {
		t.Errorf("sibling isolation broken: %v", m)
	}

	logger.InfoContext(root, "r")
	m = lastLogLine(t, buf)
	if _, has := m["shardId"]; has {
		t.Errorf("root must not see child fields: %v", m)
	}
}

// Importance 分级：Mid 字段在 Info 阈值下不出现，阈值放宽到 Debug 后出现
// （语义沿袭旧 CtxInfo：门槛看 handler 的有效阈值，不看单条日志的级别）
func TestCtxWithAttrs_Importance(t *testing.T) {
	ctx := CtxWithAttrsLevel(context.Background(), MidImportance, slog.String("cfgDump", "big"))

	loggerInfo, bufInfo, _ := newJSONLogger(slog.LevelInfo)
	loggerInfo.InfoContext(ctx, "at info threshold")
	if m := lastLogLine(t, bufInfo); m["cfgDump"] != nil {
		t.Errorf("Mid field must be omitted at Info threshold: %v", m)
	}

	loggerDebug, bufDebug, _ := newJSONLogger(slog.LevelDebug)
	loggerDebug.InfoContext(ctx, "at debug threshold")
	if m := lastLogLine(t, bufDebug); m["cfgDump"] != "big" {
		t.Errorf("Mid field must appear at Debug threshold: %v", m)
	}
}

// KLOG-010 联动：SetLevel 放宽阈值后，Mid 字段应立即开始出现
func TestCtxWithAttrs_ImportanceFollowsSetLevel(t *testing.T) {
	logger, buf, h := newJSONLogger(slog.LevelInfo)
	ctx := CtxWithAttrsLevel(context.Background(), MidImportance, slog.String("detail", "x"))

	logger.InfoContext(ctx, "before")
	if m := lastLogLine(t, buf); m["detail"] != nil {
		t.Fatalf("Mid field must be hidden before SetLevel: %v", m)
	}

	h.SetLevel(LevelDebug)
	logger.InfoContext(ctx, "after")
	if m := lastLogLine(t, buf); m["detail"] != "x" {
		t.Errorf("Mid field must appear after SetLevel(debug): %v", m)
	}
}

// KLOG-007 第 2 层：可变对象挂 ctx，各模块累加，日志在打印时刻取最新快照
type testCostCenter struct {
	dbCalls atomic.Int64
}

func (cc *testCostCenter) LogAttrs(level slog.Level) []slog.Attr {
	return []slog.Attr{slog.Int64("costDbCalls", cc.dbCalls.Load())}
}

func TestCtxWithProvider_LiveSnapshot(t *testing.T) {
	logger, buf, _ := newJSONLogger(slog.LevelInfo)
	cc := &testCostCenter{}
	ctx := CtxWithProvider(context.Background(), cc)

	cc.dbCalls.Add(2)
	logger.InfoContext(ctx, "mid-request")
	if m := lastLogLine(t, buf); m["costDbCalls"] != float64(2) {
		t.Errorf("snapshot at log time: %v", m)
	}

	cc.dbCalls.Add(3)
	logger.InfoContext(ctx, "end-request")
	if m := lastLogLine(t, buf); m["costDbCalls"] != float64(5) {
		t.Errorf("later log must see updated value: %v", m)
	}
}

// 并发：多 goroutine 同时累加 + 打日志，go test -race 必须干净
func TestCtxWithProvider_ConcurrentMutation(t *testing.T) {
	logger, _, _ := newJSONLogger(slog.LevelInfo)
	cc := &testCostCenter{}
	ctx := CtxWithProvider(context.Background(), cc)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				cc.dbCalls.Add(1)
				logger.InfoContext(ctx, "concurrent")
			}
		}()
	}
	wg.Wait()

	if got := cc.dbCalls.Load(); got != 800 {
		t.Errorf("count = %d, want 800", got)
	}
}
