package common

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
)

var (
	version     = "unknown"
	sessionId   = kcommon.RandomString(context.Background(), 8)
	startTimeMs = kcommon.GetWallTimeMs()
)

func GetVersion() string {
	return version
}

func GetSessionId() string {
	return sessionId
}

func GetStartTimeMs() int64 {
	return startTimeMs
}
