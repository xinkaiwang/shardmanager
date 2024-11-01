package klogging

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCtxInfoBasic(t *testing.T) {
	ctx := context.Background()
	var info *CtxInfo
	ctx, info = GetOrCreateCtxInfo(ctx)
	info.With("sessionId", "ABC123").With("actionType", "FetchData")

	ret := info.String()
	assert.Regexp(t, "sessionId=ABC123", ret)
	assert.Regexp(t, "actionType=FetchData", ret)
}

func f1(ctx context.Context, x int) string {
	info := GetCurrentCtxInfo(ctx)
	return info.String()
}

func f2(ctx context.Context, x int) string {
	var info *CtxInfo
	ctx, info = CreateCtxInfo(ctx)
	info.With("queryPlan", "fullScan")
	return f1(ctx, x)
}

func TestCtxInfoWithContext(t *testing.T) {
	ret := f2(context.TODO(), 0)
	expected := ", queryPlan=fullScan"
	assert.Equal(t, expected, ret)
}

func f3(ctx context.Context, x int) string {
	var info *CtxInfo
	ctx, info = CreateCtxInfo(ctx)
	info.With("sessionId", "ABC123").With("actionType", "FetchData")
	return f2(ctx, x)
}

func TestCtxInfoWithMultiLevel(t *testing.T) {
	ret := f3(context.TODO(), 0)
	assert.Regexp(t, "sessionId=ABC123", ret)
	assert.Regexp(t, "actionType=FetchData", ret)
	assert.Regexp(t, "queryPlan=fullScan", ret)
}

func TestChildCtx(t *testing.T) {
	c1 := context.TODO()
	c2, i := CreateCtxInfo(c1)
	i.With("param", "baz")
	info := GetCurrentCtxInfo(c2)
	assert.Equal(t, ", param=baz", info.ToString(DebugLevel))
}

func TestCtxInfoKeyNotFound(t *testing.T) {
	val := GetCurrentCtxInfo(context.TODO()).FindByKey("nonExistentKey", "defaultValue")
	assert.Equal(t, "defaultValue", val)
}

func TestCtxInfoMultiLevel(t *testing.T) {
	// a -> b -> c -> d -> e
	ctx := context.Background()
	ctx_a, _ := context.WithCancel(ctx)
	ctx_b, info_b := GetOrCreateCtxInfo(ctx_a)
	info_b.Name = "info_b"
	ctx_c, _ := context.WithCancel(ctx_b)
	ctx_d, info_d := CreateCtxInfo(ctx_c)
	info_d.Name = "info_d"
	ctx_e, _ := context.WithCancel(ctx_d)

	// verify
	assert.Equal(t, "info_b", GetCurrentCtxInfo(ctx_b).Name)
	assert.Equal(t, "info_d", GetCurrentCtxInfo(ctx_e).Name)
}

func TestCtxInfoExplicitParent(t *testing.T) {
	// Background() -> a -> b -> c
	ctx := context.Background()
	ctx_a, _ := context.WithCancel(ctx)
	ctx_b, info_b := GetOrCreateCtxInfo(ctx_a)
	info_b.Name = "info_b"
	info_b.With("operation", "retrieve")
	ctx_c, _ := context.WithCancel(ctx_b)

	// Background() -> d -> e
	ctx_d, _ := context.WithCancel(ctx)
	ctx_e, info_e := CreateCtxInfoWithParent(ctx_d, GetCurrentCtxInfo(ctx_c))
	info_e.Name = "info_e"
	assert.Equal(t, "info_e", GetCurrentCtxInfo(ctx_e).Name)
	assert.Equal(t, "retrieve", GetCurrentCtxInfo(ctx_e).FindByKey("operation", "")) // not the same ctx tree, but CtxInfo still in the same chain.
}

func TestCtxInfoModifyByKey(t *testing.T) {
	// Background() -> a -> b -> c
	ctx := context.Background()
	ctx_a, _ := context.WithCancel(ctx)
	ctx_b, info_b := GetOrCreateCtxInfo(ctx_a)
	info_b.With("cacheStatus", "") // unknown now, will be filled in real value later
	ctx_c, _ := CreateCtxInfo(ctx_b)

	// get value
	val := GetCurrentCtxInfo(ctx_c).FindByKey("cacheStatus", "unknown")
	assert.Equal(t, "unknown", val)

	// modify value
	GetCurrentCtxInfo(ctx_c).ModifyByKey("cacheStatus", "missed")

	// get value (again)
	val = GetCurrentCtxInfo(ctx_c).FindByKey("cacheStatus", "unknown")
	assert.Equal(t, "missed", val)

	// get value (from parent ctx)
	val = GetCurrentCtxInfo(ctx_b).FindByKey("cacheStatus", "unknown")
	assert.Equal(t, "missed", val)
}
