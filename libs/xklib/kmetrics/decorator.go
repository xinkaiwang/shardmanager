package kmetrics

import (
	"context"
	"time"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
)

var (
	OpsLatencyMetric    = CreateKmetric(context.Background(), "op_latency_ms", "desc", []string{"method", "status", "error", "notes"})
	OpsLatencyHistogram = CreateKhistogram(context.Background(), "op_lat_ms", "desc", []string{"method", "status", "error"}, []int64{1, 2, 3, 6, 10, 20, 30, 60, 100, 200, 300, 600, 1000, 2000, 3000, 6000, 10000, 20000, 30000}) // note: metrics name cannot conflict, that's why we name this `op_lat_ms`
)

// FuncTypeVoid is a function being decorated. typically this is a
// function closure that has access to variables defined in
// the enclosing scope.
// When an error happens, this func should throw (panic), that's why this func doesn't return an error.
type FuncTypeVoid func()

// FuncTypeError is a function being decorated.
// When an error happens, this func should return an error.
type FuncTypeError func(ctx context.Context) error

func invokeFuncVoid(ctx context.Context, ef FuncTypeVoid) (ke *kerror.Kerror) {
	defer func() {
		if r := recover(); r != nil {
			var ok bool
			ke, ok = r.(*kerror.Kerror)
			if !ok {
				// this is func throws a non-kerror object?!
				klogging.Fatal(ctx).WithPanic(r).Log("InvalidPanic", "invalid kerror in panic")
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
			var ok bool
			err, ok = r.(error)
			if !ok {
				// this function throws a non-error object?!
				klogging.Fatal(ctx).WithPanic(r).Log("InvalidPanic", "invalid error in panic")
			}
		}
	}()
	err = ef(ctx)
	return
}

// InstrumentSummaryRunError: helper function for adding metr
