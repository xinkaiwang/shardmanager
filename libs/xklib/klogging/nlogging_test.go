package klogging

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoggerBasic(t *testing.T) {
	SetDefaultLogger(&BasicLogger{LogLevel: DebugLevel})
	Info(context.Background()).With("userID", "User123").Log("UserNotFound", "Fetch() failed to find user")
	logLine := GetLastLoggedMessage()
	expected := "level=info, event=UserNotFound, msg=Fetch() failed to find user, userID=User123"
	assert.Equal(t, expected, logLine)
}

func TestLoggerWithCtx(t *testing.T) {
	// Prepare context with CtxInfo
	ctx := context.Background()
	ctx, info := CreateCtxInfo(ctx)
	info.
		With("sessionID", "567890").
		With("actionType", "Update")

	// Write a log entry in the new context
	Info(ctx).Log("DatabaseError", "Update() encountered a DB error")
	logLine := GetLastLoggedMessage()
	expected := "level=info, event=DatabaseError, msg=Update() encountered a DB error"
	assert.True(t, strings.HasPrefix(logLine, expected))
	assert.Regexp(t, "sessionID=567890", logLine)
	assert.Regexp(t, "actionType=Update", logLine)
}

func logFunc1(ctx context.Context, x int, t *testing.T) {
	Debug(ctx).With("attempt", x).Log("retryEvent", "Attempting retry")
	logLine := GetLastLoggedMessage()
	expected := "level=debug, event=retryEvent, msg=Attempting retry, sessionID=567890, method=POST, attempt=2"
	assert.Equal(t, expected, logLine)
}

func logFunc2(ctx context.Context, x int, t *testing.T) {
	Info(ctx).Log("validationSuccess", "Data validation passed")
	logLine := GetLastLoggedMessage()
	expected := "level=info, event=validationSuccess, msg=Data validation passed, sessionID=567890"
	assert.Equal(t, expected, logLine)

	ctx, info := CreateCtxInfo(ctx)
	info.With("method", "POST")
	logFunc1(ctx, x, t)
}

func TestLoggerWithCallStack(t *testing.T) {
	ctx, info := CreateCtxInfo(context.TODO())
	info.With("sessionID", "567890")
	logFunc2(ctx, 2, t)
}
