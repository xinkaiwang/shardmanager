package kcommon

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
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
				klogging.Fatal(ctx).WithPanic(r).Log("NonErrorPanic", "")
			}
		}
	}()
	fn()
	return
}
