package solo

import (
	"context"
	"os"
	"strings"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
)

const (
	GlobalLockPath = "/smg/global_lock"
)

type SoloMode string

const (
	// SoloModeStrict: in case we lost our etcd eph lock, we will fatal and exit
	SLM_Strict SoloMode = "strict"

	// SoloModeRobust: in case we lost our etcd eph lock, we will try to re-acquire the lock.
	// We will annunce "LockLost" only if we either 1) see someone else acquired the lock, or 2) we can not re-acquire the lock after 3 times
	SLM_Robust SoloMode = "robust"
)

func SoloModeParseFromString(modeStr string) SoloMode {
	str := strings.TrimSpace(strings.ToLower(modeStr))
	switch str {
	case string(SLM_Strict):
		return SLM_Strict
	case string(SLM_Robust):
		return SLM_Robust
	default:
		ke := kerror.Create("InvalidSoloMode", "invalid solo mode").With("value", modeStr)
		panic(ke)
	}
}

func GetSoloMode() SoloMode {
	str := os.Getenv("SMG_SOLO_MODE")
	if str == "" {
		str = string(SLM_Strict)
	}
	return SoloModeParseFromString(str)
}

type SoloManager interface {
	GetLckLostCh() <-chan struct{}
	Close(ctx context.Context)
}
