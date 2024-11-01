package kcommon

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
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
				klogging.Fatal(ctx).WithPanic(r).Log("NonErrorPanic", "")
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
	// create logEntry
	logEntry := klogging.Warning(ctx).WithError(ne)
	assert.NotNil(t, logEntry)
	assert.NotNil(t, getDetail(logEntry.Details, "stack"))
	assert.NotNil(t, getDetail(logEntry.Details, "causedBy"))
	assert.NotNil(t, getDetail(logEntry.Details, "errorType"))
	logEntry.Log("TestLogEntry", "")
}

func getDetail(details []klogging.Keypair, key string) interface{} {
	for _, detail := range details {
		if detail.K == key {
			return detail.V
		}
	}
	return nil
}
