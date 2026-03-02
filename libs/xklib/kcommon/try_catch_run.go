package kcommon

import (
	"context"
	"log/slog"
	"os"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
)

func TryCatchRun(ctx context.Context, fn func()) (ret *kerror.Kerror) {
	defer func() {
		r := recover()
		if r != nil {
			if ke, ok := r.(*kerror.Kerror); ok {
				ret = ke
			} else if err, ok := r.(error); ok {
				ret = kerror.Wrap(err, "UnknownError", "", true)
			} else {
				// we should never throw a non-error panic; this will crash this server
				slog.ErrorContext(ctx, "Non-error panic - exiting",
					slog.String("event", "NonErrorPanic"),
					slog.Any("panic", r))
				os.Exit(1)
			}
		}
	}()
	fn()
	return
}
