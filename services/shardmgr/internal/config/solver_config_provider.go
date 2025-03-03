package config

import "github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"

var (
	currentSolverConfigProvider SolverConfigProvider
)

func GetCurrentSolverConfigProvider() SolverConfigProvider {
	if currentSolverConfigProvider == nil {
		currentSolverConfigProvider = NewDefaultSolverConfigProvider()
	}
	return currentSolverConfigProvider
}

type SolverConfigProvider interface {
	SetConfig(solverConfig *smgjson.SolverConfigJson)
	GetSoftSolverConfig() *SoftSolverConfig
	GetAssignSolverConfig() *AssignSolverConfig
	GetUnassignSolverConfig() *UnassignSolverConfig
}

type DefaultSolverConfigProvider struct {
	SoftSolverConfig     SoftSolverConfig
	AssignSolverConfig   AssignSolverConfig
	UnassignSolverConfig UnassignSolverConfig
}

func NewDefaultSolverConfigProvider() *DefaultSolverConfigProvider {
	decp := &DefaultSolverConfigProvider{}
	decp.SetConfig(&smgjson.SolverConfigJson{}) // init with all default config
	return decp
}

func (dscp *DefaultSolverConfigProvider) GetSoftSolverConfig() *SoftSolverConfig {
	return &dscp.SoftSolverConfig
}

func (dscp *DefaultSolverConfigProvider) GetAssignSolverConfig() *AssignSolverConfig {
	return &dscp.AssignSolverConfig
}

func (dscp *DefaultSolverConfigProvider) GetUnassignSolverConfig() *UnassignSolverConfig {
	return &dscp.UnassignSolverConfig
}

func (dscp *DefaultSolverConfigProvider) SetConfig(solverConfig *smgjson.SolverConfigJson) {
	dscp.SoftSolverConfig = SoftSolverConfigJsonToConfig(solverConfig.SoftSolverConfig)
	dscp.AssignSolverConfig = AssignSolverConfigJsonToConfig(solverConfig.AssignSolverConfig)
	dscp.UnassignSolverConfig = UnassignSolverConfigJsonToConfig(solverConfig.UnassignSolverConfig)
}
