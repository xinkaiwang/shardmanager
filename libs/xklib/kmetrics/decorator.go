package kmetrics

import (
	"context"
	"fmt"
	"time"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
)

var (
	OpsLatencyMetric    = CreateKmetric(context.Background(), "op_latency_ms", "desc", []string{"method", "status", "error", "notes"})
	OpsLatencyHistogram = CreateKhistogram(context.Background(), "op_lat_ms", "desc", []string{"method", "status", "error"}, []int64{1, 2, 3, 6, 10, 20, 30, 60, 100, 200, 300, 600, 1000, 2000, 3000, 6000, 10000, 20000, 30000}) // note: metrics name cannot conflict, that's why we name this `op_lat_ms`
)

// FuncTypeVoid is a function being decorated.
// When an error happens, this func should throw (panic), that's why this func doesn't return an error.
type FuncTypeVoid func()

// FuncTypeError is a function being decorated.
// When an error happens, this func should return an error.
type FuncTypeError func(ctx context.Context) error

func invokeFuncVoid(ctx context.Context, ef FuncTypeVoid) (ke *kerror.Kerror) {
	defer func() {
		if r := recover(); r != nil {
			// we should print the panic type here to debug what's going on.
			fmt.Printf("panic type: %T\n", r)
			switch v := r.(type) {
			case *kerror.Kerror:
				// 已经是 kerror，直接使用
				ke = v
			case error:
				// 普通 error，包装成 kerror
				ke = kerror.Create("InternalServerError", v.Error()).
					WithErrorCode(kerror.EC_UNKNOWN)
			default:
				// 非错误的 panic 值（比如字符串或其他类型），记录 fatal 日志并退出
				klogging.Fatal(ctx).WithPanic(v).Log("InvalidPanic", "invalid panic with non-error value")
			}
		}
	}()
	ef()
	return
}

// InstrumentSummaryRunVoid: helper function for adding metrics coverage for a function that returns void.
func InstrumentSummaryRunVoid(ctx context.Context, method string, ef FuncTypeVoid, customNotes string) {
	tagMethod := method
	tagStatus := "OK"
	var tagError string

	startTime := time.Now()
	ke := invokeFuncVoid(ctx, ef) // the function closure that performs the work.
	timeSpentMs := kcommon.RoundDurationToMs(ctx, time.Since(startTime))

	if ke != nil {
		errMsg := ke.Type
		tagStatus = "ERROR"
		tagError = errMsg
	}

	OpsLatencyMetric.GetTimeSequence(ctx, tagMethod, tagStatus, tagError, customNotes).Add(timeSpentMs)
	if ke != nil {
		panic(ke)
	}
}

// Histogram is very expensive. You should use InstrumentSummaryRunVoid() instead most of the time. Only use InstrumentHistogramRunVoid() for important metrics.
func InstrumentHistogramRunVoid(ctx context.Context, method string, ef FuncTypeVoid) {
	tagMethod := method
	tagStatus := "OK"
	var tagError string

	startTime := time.Now()
	ke := invokeFuncVoid(ctx, ef)
	timeSpentMs := kcommon.RoundDurationToMs(ctx, time.Since(startTime))

	if ke != nil {
		errMsg := ke.Type
		tagStatus = "ERROR"
		tagError = errMsg
	}

	OpsLatencyHistogram.GetHistoSequence(ctx, tagMethod, tagStatus, tagError).Add(timeSpentMs)
	if ke != nil {
		panic(ke)
	}
}

func invokeFuncError(ctx context.Context, ef FuncTypeError) (err error) {
	defer func() {
		if r := recover(); r != nil {
			switch v := r.(type) {
			case *kerror.Kerror:
				// 已经是 kerror，直接使用
				err = v
			case error:
				// 普通 error，包装成 kerror
				err = kerror.Create("InternalServerError", v.Error()).
					WithErrorCode(kerror.EC_UNKNOWN)
			default:
				// 非错误的 panic 值（比如字符串或其他类型），记录 fatal 日志并退出
				klogging.Fatal(ctx).WithPanic(v).Log("InvalidPanic", "invalid panic with non-error value")
			}
		}
	}()
	err = ef(ctx)
	return
}

// InstrumentSummaryRunError: helper function for adding metr
