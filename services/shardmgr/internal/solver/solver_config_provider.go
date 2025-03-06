package solver

import (
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

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
	GetByName(solverName SolverType) *config.BaseSolverConfig
	GetSoftSolverConfig() *config.SoftSolverConfig
	GetAssignSolverConfig() *config.AssignSolverConfig
	GetUnassignSolverConfig() *config.UnassignSolverConfig
}

type DefaultSolverConfigProvider struct {
	SoftSolverConfig     config.SoftSolverConfig
	AssignSolverConfig   config.AssignSolverConfig
	UnassignSolverConfig config.UnassignSolverConfig
}

func NewDefaultSolverConfigProvider() *DefaultSolverConfigProvider {
	decp := &DefaultSolverConfigProvider{}
	decp.SetConfig(&smgjson.SolverConfigJson{}) // init with all default config
	return decp
}

func (dscp *DefaultSolverConfigProvider) GetByName(solverName SolverType) *config.BaseSolverConfig {
	switch solverName {
	case ST_SoftSolver:
		return &dscp.SoftSolverConfig.BaseSolverConfig
	case ST_AssignSolver:
		return &dscp.AssignSolverConfig.BaseSolverConfig
	case ST_UnassignSolver:
		return &dscp.UnassignSolverConfig.BaseSolverConfig
	}
	return nil
}

func (dscp *DefaultSolverConfigProvider) GetSoftSolverConfig() *config.SoftSolverConfig {
	return &dscp.SoftSolverConfig
}

func (dscp *DefaultSolverConfigProvider) GetAssignSolverConfig() *config.AssignSolverConfig {
	return &dscp.AssignSolverConfig
}

func (dscp *DefaultSolverConfigProvider) GetUnassignSolverConfig() *config.UnassignSolverConfig {
	return &dscp.UnassignSolverConfig
}

func (dscp *DefaultSolverConfigProvider) SetConfig(solverConfig *smgjson.SolverConfigJson) {
	dscp.SoftSolverConfig = config.SoftSolverConfigJsonToConfig(solverConfig.SoftSolverConfig)
	dscp.AssignSolverConfig = config.AssignSolverConfigJsonToConfig(solverConfig.AssignSolverConfig)
	dscp.UnassignSolverConfig = config.UnassignSolverConfigJsonToConfig(solverConfig.UnassignSolverConfig)
}
