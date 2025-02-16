package common

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
)

var (
	version     = "unknown"
	startId     = kcommon.RandomString(context.Background(), 8)
	startTimeMs = kcommon.GetWallTimeMs()
)

func GetVersion() string {
	return version
}

func GetStartId() string {
	return startId
}

func GetStartTimeMs() int64 {
	return startTimeMs
}
