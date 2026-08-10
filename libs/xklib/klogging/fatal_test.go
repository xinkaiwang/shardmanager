package klogging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

// KLOG-003：Fatal = log + OsExit(1)，且日志必须在退出回调触发前已经落盘
// （os.Exit 不跑 defer——临终遗言依赖 Handler 同步写）。
func TestFatal_LogsBeforeExit(t *testing.T) {
	var buf bytes.Buffer
	h := NewHandler(&HandlerOptions{Level: slog.LevelInfo, Format: "json", Output: &buf})
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(h))
	defer slog.SetDefault(oldLogger)

	oldProvider := currentOsProvider
	defer func() { currentOsProvider = oldProvider }()

	exitCode := -1
	bytesAtExit := 0
	mock := NewMockOsProvider()
	mock.ExitCb = func(code int) {
		exitCode = code
		bytesAtExit = buf.Len() // 退出瞬间日志必须已可读
	}
	mock.SetAsDefault()

	Fatal(context.Background(), "config invariant broken",
		slog.String("event", "ConfigBroken"), slog.String("key", "x"))

	if exitCode != 1 {
		t.Fatalf("Fatal must OsExit(1), got %d", exitCode)
	}
	if bytesAtExit == 0 {
		t.Fatal("fatal log must be flushed BEFORE exit (dying words lost)")
	}
	out := buf.String()
	if !strings.Contains(out, "ConfigBroken") || !strings.Contains(out, "FATAL") {
		t.Errorf("fatal line should carry event and FATAL level, got: %s", out)
	}
}

// 自定义级别渲染：FATAL/VERBOSE 输出为可读名而非 ERROR+1/DEBUG-1
func TestHandler_CustomLevelRendering(t *testing.T) {
	var buf bytes.Buffer
	h := NewHandler(&HandlerOptions{Level: LevelVerbose, Format: "json", Output: &buf})
	logger := slog.New(h)

	logger.LogAttrs(context.Background(), LevelVerbose, "v")
	if !strings.Contains(buf.String(), `"level":"VERBOSE"`) {
		t.Errorf("verbose level should render as VERBOSE, got: %s", buf.String())
	}
}
