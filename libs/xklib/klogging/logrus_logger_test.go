package klogging

import (
	"context"
	"testing"
)

func TestLogrusLoggerBasic(t *testing.T) {
	logger := NewLogrusLogger(nil)
	logger.Level()
}

func TestLogrusLoggerSetGlobal(t *testing.T) {
	SetDefaultLogger(NewLogrusLogger(nil))
	Info(context.Background()).With("x", 1).Log("EventXHappend", "this is a log message")
	// time="2021-02-02T20:45:38.186-08:00" level=info event=EventXHappend message="this is a log message" x=1
}
