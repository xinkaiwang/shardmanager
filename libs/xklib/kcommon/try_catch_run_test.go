package kcommon

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
)

// this is used only for test, with stack-enabled in returned KError
func TryCatchRunWithStack(ctx context.Context, fn func()) (ret *kerror.Kerror) {
	defer func() {
		r := recover()
		if r != nil {
			if ne, ok := r.(*kerror.Kerror); ok {
				ret = ne
			} else if err, ok := r.(error); ok {
				ret = kerror.Wrap(err, "UnknownError", "", true)
			} else {
				// we should never throw an non-error panic, this will crash this server
				slog.ErrorContext(ctx, "non-error panic - exiting",
					slog.String("event", "NonErrorPanic"),
					slog.Any("panic", r))
				os.Exit(1)
			}
		}
	}()
	fn()
	return
}

func TestError_WhenFail_ShouldLogStackTrace(t *testing.T) {
	ctx := context.Background()
	fn1 := func(x int, y int) int {
		return x + y
	}
	fn2 := func(x int, y int) int {
		return x * y
	}
	fn3 := func(x int, y int) int {
		return x / y
	}
	ne := TryCatchRunWithStack(ctx, func() {
		fn1(fn2(1, 2), fn3(0, 0))
	})
	assert.NotNil(t, ne)
	assert.IsType(t, &kerror.Kerror{}, ne) // check to be a KError
	assert.NotNil(t, ne.Stack)
	// fmt.Println(ne.Stack)
	
	// Log the error with slog
	slog.WarnContext(ctx, "test log entry",
		slog.String("event", "TestLogEntry"),
		slog.Any("error", ne),
		slog.String("stack", ne.Stack),
		slog.String("errorType", ne.Type))
	
	// Verify kerror fields exist
	assert.NotNil(t, ne.Stack)
	assert.NotNil(t, ne.Type)
	assert.NotNil(t, ne.Msg)
}
