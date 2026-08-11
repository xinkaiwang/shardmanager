package klogging

// ctx-carried ambient log fields (KLOG-007, three-layer design — see
// research/2026-08-09-ctx-info-revisit/notes.md).
//
// Layer 1 — immutable attr chain: CtxWithAttrs / CtxWithAttrsLevel attach
// fields once; every log written via slog.XxxContext(ctx, ...) below that
// point carries them. Each call adds one immutable node pointing at its
// parent — inheritance by pointer, zero copying, zero locks. Sibling ctx
// branches are isolated for free (ctx is a tree).
//
// Layer 2 — AttrProvider: a request-scoped MUTABLE object (e.g. a cost
// center) attached by pointer via CtxWithProvider. Modules mutate it at will
// (the object owns its concurrency: atomics/mutex per its access pattern);
// the Handler calls LogAttrs at log time, so every line carries the latest
// snapshot.
//
// Layer 3 — the old CtxInfo.ModifyByKey is deliberately NOT reproduced: its
// use cases map to Layer 2 (accumulate/backfill) or same-key shadowing in
// Layer 1, without the writeMutex/COW complexity it forced on everyone.
//
// Duplicate keys across layers are emitted in root→leaf order and NOT
// deduplicated (dedup would cost a map allocation per log line); JSON
// consumers conventionally keep the last occurrence, so the innermost layer
// wins on parse.

import (
	"context"
	"log/slog"
)

// Importance controls at which effective log threshold an ambient field is
// attached (semantics inherited from the old CtxInfo): the gate is the
// handler's effective threshold — NOT the individual record's level — so
// widening the threshold (config or SetLevel or trace sampling) makes
// Mid/Low fields appear on every line at once.
type Importance int

const (
	HighImportance Importance = iota // attached always
	MidImportance                    // attached when effective threshold <= Debug
	LowImportance                    // attached when effective threshold <= Verbose
)

func (imp Importance) visibleAt(threshold slog.Level) bool {
	switch imp {
	case MidImportance:
		return threshold <= LevelDebug
	case LowImportance:
		return threshold <= LevelVerbose
	default:
		return true
	}
}

// AttrProvider is a live source of log fields: LogAttrs is called at the
// moment each log line is written. Implementations must be safe for
// concurrent use (they are read from any goroutine holding the ctx).
type AttrProvider interface {
	LogAttrs(level slog.Level) []slog.Attr
}

type ctxAttrsKeyType struct{}

var ctxAttrsKey ctxAttrsKeyType

// ctxAttrsNode is one immutable layer of the ambient-field chain.
type ctxAttrsNode struct {
	parent   *ctxAttrsNode
	imp      Importance
	attrs    []slog.Attr  // static fields (Layer 1) …
	provider AttrProvider // … or a live provider (Layer 2); exactly one is set
}

func parentNode(ctx context.Context) *ctxAttrsNode {
	node, _ := ctx.Value(ctxAttrsKey).(*ctxAttrsNode)
	return node
}

// CtxWithAttrs attaches always-visible ambient fields to the ctx.
func CtxWithAttrs(ctx context.Context, attrs ...slog.Attr) context.Context {
	return CtxWithAttrsLevel(ctx, HighImportance, attrs...)
}

// CtxWithAttrsLevel attaches ambient fields gated by importance.
func CtxWithAttrsLevel(ctx context.Context, imp Importance, attrs ...slog.Attr) context.Context {
	if len(attrs) == 0 {
		return ctx
	}
	return context.WithValue(ctx, ctxAttrsKey, &ctxAttrsNode{
		parent: parentNode(ctx), imp: imp, attrs: attrs,
	})
}

// CtxWithProvider attaches a live AttrProvider (always visible).
func CtxWithProvider(ctx context.Context, p AttrProvider) context.Context {
	if p == nil {
		return ctx
	}
	return context.WithValue(ctx, ctxAttrsKey, &ctxAttrsNode{
		parent: parentNode(ctx), imp: HighImportance, provider: p,
	})
}

// appendCtxAttrs walks the chain root-first and appends visible fields to r.
func appendCtxAttrs(node *ctxAttrsNode, r *slog.Record, threshold slog.Level) {
	if node == nil {
		return
	}
	appendCtxAttrs(node.parent, r, threshold)
	if !node.imp.visibleAt(threshold) {
		return
	}
	if node.provider != nil {
		r.AddAttrs(node.provider.LogAttrs(r.Level)...)
		return
	}
	r.AddAttrs(node.attrs...)
}
