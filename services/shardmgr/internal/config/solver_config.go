package config

import "github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"

type SolverConfig struct {
	SoftSolverConfig     BaseSolverConfig
	AssignSolverConfig   BaseSolverConfig
	UnassignSolverConfig BaseSolverConfig
}

func NewSolverConfig() *SolverConfig {
	return &SolverConfig{
		SoftSolverConfig:     NewBaseSolverConfig(),
		AssignSolverConfig:   NewBaseSolverConfig(),
		UnassignSolverConfig: NewBaseSolverConfig(),
	}
}

type BaseSolverConfig struct {
	SolverEnabled bool
	RunPerMinute  int
	ExplorePerRun int
}

func NewBaseSolverConfig() BaseSolverConfig {
	return BaseSolverConfig{
		SolverEnabled: true,
		RunPerMinute:  10,
		ExplorePerRun: 10,
	}
}

func (bsc *BaseSolverConfig) ToJson() *smgjson.BaseSolverConfigJson {
	cfg := &smgjson.BaseSolverConfigJson{
		SoftSolverEnabled: nil,
		RunPerMinute:      nil,
		ExplorePerRun:     nil,
	}
	if bsc.SolverEnabled {
		cfg.SoftSolverEnabled = &bsc.SolverEnabled
	}
	if bsc.RunPerMinute > 0 {
		intVal := int32(bsc.RunPerMinute)
		cfg.RunPerMinute = &intVal
	}
	if bsc.ExplorePerRun > 0 {
		intVal := int32(bsc.ExplorePerRun)
		cfg.ExplorePerRun = &intVal
	}
	return cfg
}

// type SoftSolverConfig struct {
// 	BaseSolverConfig
// }

// type AssignSolverConfig struct {
// 	BaseSolverConfig
// }

// type UnassignSolverConfig struct {
// 	BaseSolverConfig
// }

func SolverConfigJsonToConfig(sjc *smgjson.SolverConfigJson) SolverConfig {
	if sjc == nil {
		return SolverConfig{
			SoftSolverConfig:     NewBaseSolverConfig(),
			AssignSolverConfig:   NewBaseSolverConfig(),
			UnassignSolverConfig: NewBaseSolverConfig(),
		}
	}
	cfg := SolverConfig{
		SoftSolverConfig:     BaseSolverConfigFromJson(sjc.SoftSolverConfig),
		AssignSolverConfig:   BaseSolverConfigFromJson(sjc.AssignSolverConfig),
		UnassignSolverConfig: BaseSolverConfigFromJson(sjc.UnassignSolverConfig),
	}
	return cfg
}

func (sc *SolverConfig) ToJsonObj() *smgjson.SolverConfigJson {
	cfg := &smgjson.SolverConfigJson{
		SoftSolverConfig:     sc.SoftSolverConfig.ToJson(),
		AssignSolverConfig:   sc.AssignSolverConfig.ToJson(),
		UnassignSolverConfig: sc.UnassignSolverConfig.ToJson(),
	}
	return cfg
}

func BaseSolverConfigFromJson(ssc *smgjson.BaseSolverConfigJson) BaseSolverConfig {
	cfg := BaseSolverConfig{
		SolverEnabled: false,
		RunPerMinute:  10,
		ExplorePerRun: 10,
	}
	if ssc == nil {
		return cfg
	}
	if ssc.SoftSolverEnabled != nil {
		cfg.SolverEnabled = *ssc.SoftSolverEnabled
	}
	if ssc.RunPerMinute != nil {
		cfg.RunPerMinute = int(*ssc.RunPerMinute)
	}
	if ssc.ExplorePerRun != nil {
		cfg.ExplorePerRun = int(*ssc.ExplorePerRun)
	}
	return cfg
}
