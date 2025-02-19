package common

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
)

var (
	version     = "unknown"
	sessionId   = ""
	startTimeMs = kcommon.GetWallTimeMs()
)

func GetVersion() string {
	return version
}

func GetSessionId() string {
	if sessionId == "" {
		sessionId = kcommon.RandomString(context.Background(), 8)
	}
	return sessionId
}

func GetStartTimeMs() int64 {
	return startTimeMs
}
