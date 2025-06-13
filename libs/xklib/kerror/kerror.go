package kerror

import (
	"encoding/hex"
	"fmt"
	"runtime/debug"
	"strings"
)

type Keypair struct {
	K string
	V interface{}
}

type Kerror struct {
	Type      string
	Msg       string
	Details   []Keypair // using array: map don't keep ordering
	Stack     string    // optional, normally only inner most kerror need full stack dump
	CausedBy  error     // optional, maybe a *Kerror, or also possible just an error
	ErrorCode ErrorCode // optional, default=UNKNOWN(2), grpc code from https://github.com/googleapis/googleapis/blob/master/google/rpc/code.proto
}

func Create(errType string, msg string) *Kerror {
	return &Kerror{
		Stack:     GetCallStack(1),
		Type:      errType,
		Msg:       msg,
		ErrorCode: EC_UNKNOWN,
	}
}

func (ke *Kerror) Error() string {
	return ke.ShortString()
}

func (ke *Kerror) String() string {
	return ke.FullString()
}

func (ke *Kerror) With(key string, val interface{}) *Kerror {
	ke.Details = append(ke.Details, Keypair{K: key, V: val})
	return ke
}

func (ke *Kerror) WithErrorCode(code ErrorCode) *Kerror {
	ke.ErrorCode = code
	return ke
}

// to make Kerror work with "errors.Is()", "errors.As()"... standard operations
func (ke *Kerror) Unwrap() error {
	return ke.CausedBy
}

func (ke *Kerror) WithoutStack() *Kerror {
	ke.Stack = "" // clear stack trace
	return ke
}

func (ke *Kerror) GetType() string {
	return ke.Type
}

func (ke *Kerror) ShortString() string {
	var b strings.Builder
	b.Grow(1000)
	ke.ToFullString(&b, false /*withStack*/, false /*withCause*/)
	return b.String()
}

func (ke *Kerror) FullString() string {
	var b strings.Builder
	b.Grow(1000)
	ke.ToFullString(&b, true /*withStack*/, true /*withCause*/)
	return b.String()
}

func (ke *Kerror) CausedByString() string {
	var b strings.Builder
	b.Grow(1000)
	ke.buildCausedByString(&b, false /*withStack*/, true /*withCause*/)
	return b.String()
}

func (ke *Kerror) ToFullString(b *strings.Builder, withStack, withCause bool) {
	// current
	fmt.Fprintf(b, "%s: %s", ke.Type, ke.Msg)
	// Details
	for _, item := range ke.Details {
		fmt.Fprintf(b, ", %s=%v", item.K, formatVal(item.V))
	}
	// stack trace
	if withStack {
		if ke.Stack != "" {
			fmt.Fprintf(b, ", stack=%s", ke.Stack)
		}
	}
	// caused by
	if withCause {
		fmt.Fprintf(b, ";\n Caused by: ")
		ke.buildCausedByString(b, withStack, withCause)
		fmt.Fprintf(b, "\n")
	}
}

func (ke *Kerror) buildCausedByString(b *strings.Builder, withStack, withCause bool) {
	if ke.CausedBy != nil {
		if cause, ok := ke.CausedBy.(*Kerror); ok { // it's a Kerror?
			cause.ToFullString(b, withStack, withCause)
		} else { // nop, it's just an generic error
			fmt.Fprintf(b, "%s", ke.CausedBy.Error())
		}
	}
}

func (ke *Kerror) GetHttpErrorCode() int {
	return ke.ErrorCode.ToHttpErrorCode()
}

func formatVal(val interface{}) interface{} {
	if val == nil {
		return nil
	} else if bytes, ok := val.([]byte); ok {
		return hex.EncodeToString(bytes)
	} else {
		return val
	}
}

func GetCallStack(removeTop int) string {
	stack := string(debug.Stack())
	// fmt.Print(stack)
	// skip first few lines, last element is everything else
	split := strings.SplitAfterN(stack, "\n", 6+2*removeTop)
	return split[len(split)-1]
}

// Note: stackTrace is expensive. So you should only attach stack when really needed.
func Wrap(err error, errType, msg string, needStack bool) *Kerror {
	ke := &Kerror{
		Type:      errType,
		Msg:       msg,
		CausedBy:  err,
		ErrorCode: EC_UNKNOWN,
	}

	if Retryable(err) {
		ke.ErrorCode = EC_RETRYABLE
	}
	if needStack {
		// add stack trace only if inner error is not an instance of Kerror
		if _, ok := err.(*Kerror); !ok {
			ke.Stack = GetCallStack(1)
		}
	}
	return ke
}

// ******************** Retryable ********************
type retryable interface {
	Retryable() bool
}

// implement interface retryable
func (ke *Kerror) Retryable() bool {
	return ke.ErrorCode.String() == string(EC_RETRYABLE)
}

// Retryable: use this to verify a given error (not necessarily a Kerror) is retryable or not.
func Retryable(err error) bool {
	if err == nil {
		return false
	}
	retry, ok := err.(retryable)
	if !ok {
		return false
	}
	return retry.Retryable()
}
