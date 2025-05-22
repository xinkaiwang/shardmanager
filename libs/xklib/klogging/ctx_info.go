package klogging

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
)

// key is an unexported type for keys defined in this package.
// This prevents collisions with keys defined in other packages.
type key int

// userKey is the key for CtxInfo values in Contexts. It is
// unexported; clients use NewContext and FromContext
// instead of using this key directly.
var userKey key

// Level type
type Importance uint32

// Note: all those log-levels borrowed from logrus (simplified by removing "PanicLevel", and rename to "VerboseLevel")
const (
	// High Importance info should get included in all log events
	HighImportance Importance = 1
	// Mid Importance should get included in Debug level events
	MidImportance = 5
	// Low Importance should get included in Verbose level events
	LowImportance = 6
)

type KVL struct {
	K string
	V string
	L Importance
}

type CtxInfo struct {
	Name       string // for test/debug/log use only
	Parent     *CtxInfo
	Details    map[string]*KVL
	writeMutex sync.Mutex // protect Details from concurrent write
}

func NewCtxInfo(parent *CtxInfo) *CtxInfo {
	return &CtxInfo{
		Parent:  parent,
		Details: map[string]*KVL{},
	}
}

// CurrentCtxInfo returns the CtxInfo stored in ctx, if any.
// Return nil if current ctx does not contains an CtxInfo object
func GetCurrentCtxInfo(ctx context.Context) *CtxInfo {
	if ctx == nil {
		return nil
	}
	u, _ := ctx.Value(userKey).(*CtxInfo)
	return u
}

// Create a new child info, using current ctx as parent.
func CreateCtxInfo(ctx context.Context) (context.Context, *CtxInfo) {
	info := &CtxInfo{
		Parent:  GetCurrentCtxInfo(ctx),
		Details: map[string]*KVL{},
	}
	newCtx := context.WithValue(ctx, userKey, info)
	return newCtx, info
}

func AttachToCtx(ctx context.Context, ci *CtxInfo) context.Context {
	if ci == nil {
		return ctx
	}
	newCtx := context.WithValue(ctx, userKey, ci)
	return newCtx
}

// GetOrCreateCtxInfo: this is rarely used, most of the time you need GetOrCreateCtxInfo(), which automatically refers to parents for you. You only need this when (in rare cases) you want to attach CtxInfo onto a different ctx chain (which is not in the same ctx chain).
// get current CtxInfo (if already exists in ctx), otherwise create a new CtxInfo as well as new ctx
func GetOrCreateCtxInfo(ctx context.Context) (context.Context, *CtxInfo) {
	info := GetCurrentCtxInfo(ctx)
	if info != nil {
		return ctx, info
	}
	return CreateCtxInfo(ctx)
}

// CreateCtxInfoWithParent: this is rarely used, most of the time you need GetOrCreateCtxInfo(), which automatically refers to parents for you. You only need this when (in rare cases) you want to attach CtxInfo onto a different ctx chain (which is not in the same ctx chain).
// Create a new child info, explicit parent
func CreateCtxInfoWithParent(ctx context.Context, parent *CtxInfo) (context.Context, *CtxInfo) {
	info := &CtxInfo{
		Parent:  parent,
		Details: map[string]*KVL{},
	}
	newCtx := context.WithValue(ctx, userKey, info)
	return newCtx, info
}

// Adding values into info
func (info *CtxInfo) With(k string, v string) *CtxInfo {
	info.Details[k] = &KVL{k, v, HighImportance}
	return info
}

// Adding values into info
func (info *CtxInfo) WithLevel(k string, v string, level Importance) *CtxInfo {
	info.Details[k] = &KVL{k, v, level}
	return info
}

func (info *CtxInfo) ToString(threshold Level) string {
	var b strings.Builder
	b.Grow(1000)
	info.VisitForward(func(k string, v string) bool {
		fmt.Fprintf(&b, ", %s=%v", k, v)
		return true
	}, threshold)
	return b.String()
}

func (info *CtxInfo) String() string {
	return info.ToString(InfoLevel)
}

func (info *CtxInfo) GetAllValuesAsMap(threshold Level) map[string]string {
	data := make(map[string]string, 8)
	info.VisitForward(func(k string, v string) bool {
		data[k] = v
		return true
	}, threshold)
	return data
}

func importance2LoggingLevel(imp Importance) Level {
	switch imp {
	case HighImportance:
		return FatalLevel
	case MidImportance:
		return DebugLevel
	case LowImportance:
		return VerboseLevel
	default:
		ke := kerror.Create("unknownImportanceLevel", "").With("importance", int(imp))
		panic(ke)
	}
}

// VisitForward: visit top-level parents first, then its child, until leaf child.
// visitor: lambda function to visit each k-v, return false means satisfied, we can early terminate this visit.
// VisitForward returns false if this visit is an early-terminate.
func (info *CtxInfo) VisitForward(visitor func(k string, v string) bool, threshold Level) bool {
	if info == nil {
		return true
	}

	if !info.Parent.VisitForward(visitor, threshold) {
		return false
	}

	for _, item := range info.Details {
		if NeedLog(importance2LoggingLevel(item.L), threshold) {
			if item.V == "" {
				continue
			}
			if !visitor(item.K, item.V) {
				// false means early terminate, in that case no need to visit more data
				return false
			}
		}
	}
	return true
}

// return fallback string if not found
func (info *CtxInfo) FindByKey(k string, fallback string) (v string) {
	if info == nil {
		return fallback
	}
	val, ok := info.Details[k]
	if ok {
		if val.V != "" {
			return val.V
		} else {
			return fallback
		}
	}
	return info.Parent.FindByKey(k, fallback)
}

// ModifyByKey: find and replace value (only replace the first match encountered in the recursion)
// return false when the key is not found in this and all parent info
// Uses a write lock to avoid concurrent writes
// Uses atomic write to avoid concurrent map read/write issues
func (info *CtxInfo) ModifyByKey(k string, v string) bool {
	if info == nil {
		return false
	}
	// check current node
	_, ok := info.Details[k]
	if ok {
		info.blockingModifyKey(k, v)
		return true
	}

	// (not found so far) let's explore parent nodes
	return info.Parent.ModifyByKey(k, v)
}

// Avoid concurrent map write/iterate by using a write lock and overwriting Details atomically.
func (info *CtxInfo) blockingModifyKey(k string, v string) {
	if info == nil {
		return
	}
	// Avoid concurrent write
	info.writeMutex.Lock()
	defer info.writeMutex.Unlock()

	newDetails := make(map[string]*KVL, len(info.Details))

	for key := range info.Details {
		newDetails[key] = info.Details[key]
	}
	newDetails[k].V = v
	info.Details = newDetails
}
