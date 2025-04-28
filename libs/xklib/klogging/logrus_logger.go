package klogging

// Note: LogrusLogger in its own package (instead of klogging) is to avoid loop dependency
// LogrusLogger depends on middleware. middleware depends on klogging

import (
	"context"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
)

/*
logrus_logger implements klogging.Logger interface, and under the hood it's using logrus to do those logging and formatting.
*/

// LogrusLogger implements klogging.Logger.
type LogrusLogger struct {
	ctx             context.Context
	NextLogger      Logger // mostly nil, for exp, you may want to chain up another splunkLogger here to post a copy of logs to splunk HEC.
	RusLogger       *logrus.Logger
	logLevel        Level
	logFormat       LogFormat
	metricsReporter LoggerMetrcsReporter
}

const (
	// TimestampFormat:
	// 0) easy to read by human, no ambiguity.
	// 1) need to have 3-digits after seconds (to ms resolution)
	// 2) need to have timezone info (easy to read)
	// 3) need to be sorting friendly (2006-01-02, instead of Jan.02.2006)
	TimestampFormat = "2006-01-02T15:04:05.999Z07:00"
)

type LoggerMetrcsReporter interface {
	ReportLogSizeBytes(ctx context.Context, size int, logLevel, eventType string)
	ReportLogErrorCount(ctx context.Context, count int, logLevel, eventType string, isLogged bool)
}

// NewLogrusLogger create a new *LogrusLogger instance
func NewLogrusLogger(ctx context.Context) *LogrusLogger {
	if ctx == nil {
		ctx = context.Background()
	}
	log := logrus.New()
	log.SetFormatter(&logrus.TextFormatter{
		DisableColors:   true,
		TimestampFormat: TimestampFormat,
		FullTimestamp:   true,
	})
	logLevel := InfoLevel
	// log level threshold is evaluated in the LogrusLogger layer, not here. In RusLogger we just blindly accept everything.
	log.SetLevel(logrus.TraceLevel)
	return &LogrusLogger{
		ctx:       ctx,
		RusLogger: log,
		logLevel:  logLevel,
		logFormat: TextFormat,
	}
}

func (logger *LogrusLogger) WithNextLogger(next Logger) *LogrusLogger {
	logger.NextLogger = next
	return logger
}

func (logger *LogrusLogger) WithMetricsReporter(reporter LoggerMetrcsReporter) *LogrusLogger {
	logger.metricsReporter = reporter
	return logger
}

// Log format
type LogFormat uint32

const (
	TextFormat LogFormat = iota + 1
	JsonFormat
	SimpleFormat
)

func (e LogFormat) String() string {
	switch e {
	case TextFormat:
		return "Text"
	case JsonFormat:
		return "Json"
	case SimpleFormat:
		return "Simple"
	default:
		return fmt.Sprintf("%d", int(e))
	}
}

// may throw if unable to parse
func parseLogFormat(str string) LogFormat {
	if strings.EqualFold("text", str) {
		return TextFormat
	} else if strings.EqualFold("json", str) {
		return JsonFormat
	} else if strings.EqualFold("simple", str) {
		return SimpleFormat
	}
	panic(kerror.Create("UnknownLogFormat", "parse log format failed").With("str", str))
}

// LogLevel: fatal, error, warning, info, debug, verbose
// Format: text, json
func (logger *LogrusLogger) SetConfig(ctx context.Context, newLevelStr string, newFormatStr string) *LogrusLogger {
	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(error); ok {
				Warning(ctx).WithError(err).Log("updateLogConfigFailed", "LogConfig update failed")
			} else {
				Warning(ctx).With("err", r).Log("updateLogConfigFailed", "LogConfig update failed")
			}
		}
	}()
	// logLevel
	newLevel := ParseLogLevel(newLevelStr)
	if logger.logLevel != newLevel {
		Info(ctx).With("oldLogLevel", logger.logLevel).With("newLogLevel", newLevel).Log("updateLogLevel", "LogLevel updated")
		logger.logLevel = newLevel
	}
	// logFormat
	newFormat := parseLogFormat(newFormatStr)
	if logger.logFormat != newFormat {
		switch newFormat {
		case TextFormat:
			{
				logger.RusLogger.SetFormatter(&logrus.TextFormatter{
					DisableColors:   false,
					TimestampFormat: TimestampFormat,
					FullTimestamp:   true,
				})
			}
		case JsonFormat:
			{
				logger.RusLogger.SetFormatter(&logrus.JSONFormatter{
					TimestampFormat: TimestampFormat,
				})
			}
		case SimpleFormat:
			{
				logger.RusLogger.SetFormatter(NewSimpleFormatter())
			}
		default:
			{
				panic(kerror.Create("UnsupportedLogFormat", "unsupported log format").With("newLogFormat", newFormat))
			}
		}
		Info(ctx).With("oldLogFormat", logger.logFormat).With("newLogFormat", newFormat).Log("updateLogFormat", "LogFormat updated")
		logger.logFormat = newFormat
	}
	return logger
}

func estimateLength(obj interface{}) int {
	if str, ok := obj.(fmt.Stringer); ok {
		return len(str.String())
	} else {
		return len(fmt.Sprintf("%+v", obj))
	}
}

// Log: override interface klogging.Logger
// when shouldLog=false, we will not write log message, but we still need to report metrics count.
func (logger *LogrusLogger) Log(entry *LogEntry, shouldLog bool) {
	fields := make(map[string]interface{}, len(entry.Details))
	// metrics
	logSize := len(entry.Msg) + len(entry.LogType)
	for _, item := range entry.Details {
		logSize += len(item.K)
		logSize += estimateLength(item.V)
		fields[item.K] = item.V
	}
	if logger.metricsReporter != nil && shouldLog {
		logger.metricsReporter.ReportLogSizeBytes(logger.ctx, logSize, entry.Level.String(), entry.LogType)
	}

	if NeedLog(entry.Level, DebugLevel) {
		// here we only count debug/info/warn/error/fatal events
		if logger.metricsReporter != nil {
			logger.metricsReporter.ReportLogErrorCount(logger.ctx, 1, entry.Level.String(), entry.LogType, shouldLog)
		}
	}

	// Logrus
	if !shouldLog {
		return
	}
	// logrus fields
	for _, item := range entry.Details {
		fields[item.K] = item.V
	}
	ent := logger.RusLogger.WithField("event", entry.LogType).WithFields(fields)
	ent.Time = entry.Timestamp
	ent.Log(kloggingLevel2Logrus(entry.Level), entry.Msg)

	// otherLogger (logger chain)
	if logger.NextLogger != nil {
		logger.NextLogger.Log(entry, shouldLog)
	}
}

// LogrusLevel2klogging: convert (logrus) Level to (klogging) Level
func logrusLevel2klogging(level logrus.Level) Level {
	n := int(level)
	if n < int(FatalLevel) { // klogging don't have PanicLevel(0)
		return FatalLevel
	} else {
		return Level(n)
	}
}

// kloggingLevel2Logrus: convert (klogging) Level to (logrus) Level
func kloggingLevel2Logrus(level Level) logrus.Level {
	return logrus.Level(int(level))
}

// Level: override interface klogging.Logger
func (logger *LogrusLogger) Level() Level {
	return logger.logLevel
}
