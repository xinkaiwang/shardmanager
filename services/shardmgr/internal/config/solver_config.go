package config

import "github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"

type SolverConfig struct {
	SoftSolverConfig     SoftSolverConfig
	AssignSolverConfig   AssignSolverConfig
	UnassignSolverConfig UnassignSolverConfig
}

type BaseSolverConfig struct {
	SolverEnabled bool
	RunPerMinute  int
	ExplorePerRun int
}

type SoftSolverConfig struct {
	BaseSolverConfig
}

type AssignSolverConfig struct {
	BaseSolverConfig
}

type UnassignSolverConfig struct {
	BaseSolverConfig
}

func SolverConfigJsonToConfig(sjc *smgjson.SolverConfigJson) SolverConfig {
	cfg := SolverConfig{
		SoftSolverConfig:     SoftSolverConfigJsonToConfig(sjc.SoftSolverConfig),
		AssignSolverConfig:   AssignSolverConfigJsonToConfig(sjc.AssignSolverConfig),
		UnassignSolverConfig: UnassignSolverConfigJsonToConfig(sjc.UnassignSolverConfig),
	}
	return cfg
}

func SoftSolverConfigJsonToConfig(ssc *smgjson.SoftSolverConfigJson) SoftSolverConfig {
	cfg := SoftSolverConfig{
		BaseSolverConfig: BaseSolverConfig{
			SolverEnabled: false,
			RunPerMinute:  10,
			ExplorePerRun: 10,
		},
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

func AssignSolverConfigJsonToConfig(asc *smgjson.AssignSolverConfigJson) AssignSolverConfig {
	cfg := AssignSolverConfig{
		BaseSolverConfig: BaseSolverConfig{
			SolverEnabled: false,
			RunPerMinute:  10,
			ExplorePerRun: 10,
		},
	}
	if asc == nil {
		return cfg
	}
	if asc.AssignSolverEnabled != nil {
		cfg.SolverEnabled = *asc.AssignSolverEnabled
	}
	if asc.RunPerMinute != nil {
		cfg.RunPerMinute = int(*asc.RunPerMinute)
	}
	if asc.ExplorePerRun != nil {
		cfg.ExplorePerRun = int(*asc.ExplorePerRun)
	}
	return cfg
}

func UnassignSolverConfigJsonToConfig(usc *smgjson.UnassignSolverConfigJson) UnassignSolverConfig {
	cfg := UnassignSolverConfig{
		BaseSolverConfig: BaseSolverConfig{
			SolverEnabled: false,
			RunPerMinute:  10,
			ExplorePerRun: 10,
		},
	}
	if usc == nil {
		return cfg
	}
	if usc.UnassignSolverEnabled != nil {
		cfg.SolverEnabled = *usc.UnassignSolverEnabled
	}
	if usc.RunPerMinute != nil {
		cfg.RunPerMinute = int(*usc.RunPerMinute)
	}
	if usc.ExplorePerRun != nil {
		cfg.ExplorePerRun = int(*usc.ExplorePerRun)
	}
	return cfg
}
