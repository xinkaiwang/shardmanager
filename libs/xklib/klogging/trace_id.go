package klogging

import (
	"context"
)

func EmbedTraceId(ctx context.Context, traceId string) context.Context {
	parent := GetCurrentCtxInfo(ctx)
	ci := NewCtxInfo(parent)
	ctx2 := AttachToCtx(ctx, ci)
	ci.With("traceId", traceId)
	return ctx2
}
