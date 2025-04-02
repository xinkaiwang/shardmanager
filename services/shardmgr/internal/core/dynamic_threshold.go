package core

import (
	"math"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
)

type DynamicThreshold struct {
	configProvider config.DynamicThresholdConfigProvider
	currentTimeMs  int64
	threshold      float64
}

func NewDynamicThreshold(cfgProvider config.DynamicThresholdConfigProvider) *DynamicThreshold {
	cfg := cfgProvider()
	return &DynamicThreshold{
		configProvider: cfgProvider,
		currentTimeMs:  kcommon.GetWallTimeMs(),
		threshold:      float64(cfg.DynamicThresholdMin),
	}
}

func (dt *DynamicThreshold) GetCurrentThreshold(currentTimeMs int64) float64 {
	// Update the current time
	if currentTimeMs < dt.currentTimeMs { // this should not happen
		return dt.threshold
	}
	if currentTimeMs == dt.currentTimeMs {
		return dt.threshold
	}
	cfg := dt.configProvider()
	if dt.threshold > float64(cfg.DynamicThresholdMax) {
		dt.threshold = float64(cfg.DynamicThresholdMax)
	}
	if dt.threshold < float64(cfg.DynamicThresholdMin) {
		dt.threshold = float64(cfg.DynamicThresholdMin)
	}
	elapsedMs := currentTimeMs - dt.currentTimeMs
	value := dt.threshold - float64(cfg.DynamicThresholdMin)
	halfDecayMs := float64(cfg.HalfDecayTimeSec) * 1000.0
	// given the elapsed time, value should decay acroding the half decay time under power law
	dt.threshold = float64(cfg.DynamicThresholdMin) + value*math.Pow(0.5, float64(elapsedMs)/halfDecayMs)
	dt.currentTimeMs = currentTimeMs
	return dt.threshold
}

func (dt *DynamicThreshold) UpdateThreshold(currentTimeMs int64, moveCount int) {
	cfg := dt.configProvider()
	if moveCount <= 0 {
		return
	}
	dt.GetCurrentThreshold(currentTimeMs)
	dt.threshold += float64(moveCount) * float64(cfg.IncreasePerMove)
	if dt.threshold > float64(cfg.DynamicThresholdMax) {
		dt.threshold = float64(cfg.DynamicThresholdMax)
	}
}
