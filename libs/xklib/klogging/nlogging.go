package klogging

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"go.opencensus.io/tag"
)

// Level type
type Level uint32

var (
	TimeProvider func() time.Time
)

// Original log levels kept intact
const (
	FatalLevel Level = iota + 1
	ErrorLevel
	WarnLevel
	InfoLevel
	DebugLevel
	VerboseLevel
)

var (
	KLogLevel  = tag.MustNewKey("level")
	KEventType = tag.MustNewKey("event")
	KLogged    = tag.MustNewKey("logged")
)

func (e Level) String() string {
	switch e {
	case FatalLevel:
		return "fatal"
	case ErrorLevel:
		return "error"
	case WarnLevel:
		return "warn"
	case InfoLevel:
		return "info"
	case DebugLevel:
		return "debug"
	case VerboseLevel:
		return "verbose"
	default:
		return fmt.Sprintf("%d", int(e))
	}
}

func ParseLogLevel(str string) Level {
	switch {
	case strings.EqualFold("fatal", str):
		return FatalLevel
	case strings.EqualFold("error", str) || strings.EqualFold("err", str):
		return ErrorLevel
	case strings.EqualFold("warning", str) || strings.EqualFold("warn", str):
		return WarnLevel
	case strings.EqualFold("information", str) || strings.EqualFold("info", str):
		return InfoLevel
	case strings.EqualFold("debug", str):
		return DebugLevel
	case strings.EqualFold("verbose", str) || strings.EqualFold("trace", str):
		return VerboseLevel
	default:
		panic(kerror.Create("UnknownLogLevel", "parse log level failed").With("str", str))
	}
}

func intInRange(val, min, max int) int {
	if val < min {
		return min
	} else if val > max {
		return max
	}
	return val
}

func (level Level) levelBoost(n int) Level {
	current := int(level)
	current = intInRange(current-n, int(FatalLevel), int(VerboseLevel))
	return Level(current)
}

func (level Level) MoreVerbose() Level {
	return level.levelBoost(-1)
}

func (level Level) LessVerbose() Level {
	return level.levelBoost(1)
}

func NeedLog(importance Level, threshold Level) bool {
	return int(importance) <= int(threshold)
}

type Logger interface {
	Log(entry *LogEntry, shouldLog bool)
	Level() Level
}

type LoggerHolder struct {
	Logger Logger
}

var currentLogger atomic.Value

func getCurrentLogger() Logger {
	val := currentLogger.Load()
	if l, ok := val.(*LoggerHolder); ok {
		return l.Logger
	}
	return nil
}

func GetLogger() Logger {
	current := getCurrentLogger()
	if current == nil {
		current = &BasicLogger{
			LogLevel: DebugLevel,
		}
		currentLogger.Store(&LoggerHolder{current})
	}
	return current
}

func SetDefaultLogger(logger Logger) {
	currentLogger.Store(&LoggerHolder{logger})
}

type Keypair struct {
	K string
	V interface{}
}

type LogEntry struct {
	Logger             Logger
	Level              Level
	EffectiveThreshold Level
	ShouldLog          bool
	LogType            string
	Msg                string
	Details            []Keypair
	Ctx                context.Context
	Timestamp          time.Time
}

func NewEntry(ctx context.Context, level Level) *LogEntry {
	logger := GetLogger()
	threshold := logger.Level()
	var now time.Time
	if TimeProvider != nil {
		now = TimeProvider()
	} else {
		now = time.Now()
	}
	entry := &LogEntry{
		Logger:             logger,
		Level:              level,
		EffectiveThreshold: threshold,
		ShouldLog:          NeedLog(level, threshold),
		Ctx:                ctx,
		Timestamp:          now,
	}
	if entry.ShouldLog {
		info := GetCurrentCtxInfo(ctx)
		if info != nil {
			info.VisitForward(func(k, v string) bool {
				entry.Details = append(entry.Details, Keypair{k, v})
				return true
			}, threshold)
		}
	}
	return entry
}

func (entry *LogEntry) With(k string, v interface{}) *LogEntry {
	if entry.ShouldLog {
		entry.Details = append(entry.Details, Keypair{k, v})
	}
	return entry
}

// this is to avoid fake nil pointer problem
func IsNil(i interface{}) bool {
	if i == nil {
		return true
	}
	switch reflect.TypeOf(i).Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func:
		return reflect.ValueOf(i).IsNil()
	}
	return false
}

func (entry *LogEntry) WithError(err error) *LogEntry {
	if !entry.ShouldLog {
		return entry
	}
	if IsNil(err) {
		return entry
	}
	if ke, ok := err.(*kerror.Kerror); ok {
		for _, item := range ke.Details {
			entry.Details = append(entry.Details, Keypair{item.K, item.V})
		}
		entry.Details = append(entry.Details, Keypair{"errorType", ke.Type}, Keypair{"errorMsg", ke.Msg}, Keypair{"stack", ke.Stack}, Keypair{"causedBy", ke.CausedByString()})
	} else {
		entry.Details = append(entry.Details, Keypair{"error", err.Error()}, Keypair{"stack", kerror.GetCallStack(1)})
	}
	return entry
}

func (entry *LogEntry) WithPanic(r interface{}) *LogEntry {
	if ne, ok := r.(*kerror.Kerror); ok {
		entry.WithError(ne)
		// include full stack trace
		entry.With("stack", ne.Stack)
	} else if err, ok := r.(error); ok {
		entry.WithError(err)
		entry.With("stack", kerror.GetCallStack(1))
	} else {
		entry.With("panic", r)
		entry.With("stack", kerror.GetCallStack(1))
	}
	return entry
}

func (entry *LogEntry) Log(logType, msg string) {
	entry.LogType = logType
	entry.Msg = msg
	entry.Logger.Log(entry, entry.ShouldLog)
	if entry.Level == FatalLevel {
		OsExit(1)
	}
}

func (entry *LogEntry) String() string {
	var b strings.Builder
	b.Grow(1000)
	fmt.Fprintf(&b, "level=%v, event=%s, msg=%s", entry.Level.String(), entry.LogType, entry.Msg)
	for _, item := range entry.Details {
		fmt.Fprintf(&b, ", %s=%v", item.K, item.V)
	}
	return b.String()
}

func Fatal(ctx context.Context) *LogEntry {
	return NewEntry(ctx, FatalLevel)
}
func Error(ctx context.Context) *LogEntry {
	return NewEntry(ctx, ErrorLevel)
}
func Warning(ctx context.Context) *LogEntry {
	return NewEntry(ctx, WarnLevel)
}
func Info(ctx context.Context) *LogEntry {
	return NewEntry(ctx, InfoLevel)
}
func Debug(ctx context.Context) *LogEntry {
	return NewEntry(ctx, DebugLevel)
}
func Verbose(ctx context.Context) *LogEntry {
	return NewEntry(ctx, VerboseLevel)
}

/********************************* BasicLogger ************************************/

type BasicLogger struct {
	LogLevel Level
}

// 添加一个互斥锁来保护lastLoggedMessage
var lastLoggedMessageMutex sync.Mutex
var lastLoggedMessage string

func (bl *BasicLogger) Log(entry *LogEntry, shouldLog bool) {
	if shouldLog {
		fmt.Println(entry.String())

		// 使用互斥锁保护对lastLoggedMessage的访问
		lastLoggedMessageMutex.Lock()
		lastLoggedMessage = entry.String()
		lastLoggedMessageMutex.Unlock()
	}
}

func (bl *BasicLogger) Level() Level {
	return bl.LogLevel
}

func GetLastLoggedMessage() string {
	// 使用互斥锁保护对lastLoggedMessage的读取
	lastLoggedMessageMutex.Lock()
	defer lastLoggedMessageMutex.Unlock()
	return lastLoggedMessage
}

// GetDefaultLogger returns the current default logger
func GetDefaultLogger() Logger {
	return getCurrentLogger()
}

// NullLogger is a logger that discards all log entries
type NullLogger struct{}

func (nl *NullLogger) Log(entry *LogEntry, shouldLog bool) {}

func (nl *NullLogger) Level() Level {
	return VerboseLevel
}

// NewNullLogger creates a new NullLogger that discards all log entries
func NewNullLogger() Logger {
	return &NullLogger{}
}
