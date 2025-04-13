package solver

import (
	"sync"

	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

var (
	currentSolverConfigProvider SolverConfigProvider
	solverConfigProviderMutex   sync.RWMutex
)

func GetCurrentSolverConfigProvider() SolverConfigProvider {
	solverConfigProviderMutex.RLock()
	provider := currentSolverConfigProvider
	solverConfigProviderMutex.RUnlock()

	if provider == nil {
		solverConfigProviderMutex.Lock()
		if currentSolverConfigProvider == nil {
			currentSolverConfigProvider = NewDefaultSolverConfigProvider()
		}
		provider = currentSolverConfigProvider
		solverConfigProviderMutex.Unlock()
	}
	return provider
}

func RunWithSolverConfigProvider(solverConfigProvider SolverConfigProvider, f func()) {
	solverConfigProviderMutex.Lock()
	oldProvider := currentSolverConfigProvider
	currentSolverConfigProvider = solverConfigProvider
	solverConfigProviderMutex.Unlock()

	defer func() {
		solverConfigProviderMutex.Lock()
		currentSolverConfigProvider = oldProvider
		solverConfigProviderMutex.Unlock()
	}()
	f()
}

type SolverConfigProvider interface {
	SetConfig(solverConfig *smgjson.SolverConfigJson)
	GetByName(solverName SolverType) *config.BaseSolverConfig
	GetSoftSolverConfig() *config.BaseSolverConfig
	GetAssignSolverConfig() *config.BaseSolverConfig
	GetUnassignSolverConfig() *config.BaseSolverConfig
}

type DefaultSolverConfigProvider struct {
	mu                   sync.RWMutex // 保护对配置的访问
	SoftSolverConfig     config.BaseSolverConfig
	AssignSolverConfig   config.BaseSolverConfig
	UnassignSolverConfig config.BaseSolverConfig
}

func NewDefaultSolverConfigProvider() *DefaultSolverConfigProvider {
	decp := &DefaultSolverConfigProvider{}
	decp.SetConfig(&smgjson.SolverConfigJson{}) // init with all default config
	return decp
}

func (dscp *DefaultSolverConfigProvider) GetByName(solverName SolverType) *config.BaseSolverConfig {
	dscp.mu.RLock()
	defer dscp.mu.RUnlock()

	switch solverName {
	case ST_SoftSolver:
		return &dscp.SoftSolverConfig
	case ST_AssignSolver:
		return &dscp.AssignSolverConfig
	case ST_UnassignSolver:
		return &dscp.UnassignSolverConfig
	}
	return nil
}

func (dscp *DefaultSolverConfigProvider) GetSoftSolverConfig() *config.BaseSolverConfig {
	dscp.mu.RLock()
	defer dscp.mu.RUnlock()
	return &dscp.SoftSolverConfig
}

func (dscp *DefaultSolverConfigProvider) GetAssignSolverConfig() *config.BaseSolverConfig {
	dscp.mu.RLock()
	defer dscp.mu.RUnlock()
	return &dscp.AssignSolverConfig
}

func (dscp *DefaultSolverConfigProvider) GetUnassignSolverConfig() *config.BaseSolverConfig {
	dscp.mu.RLock()
	defer dscp.mu.RUnlock()
	return &dscp.UnassignSolverConfig
}

func (dscp *DefaultSolverConfigProvider) SetConfig(solverConfig *smgjson.SolverConfigJson) {
	dscp.mu.Lock()
	defer dscp.mu.Unlock()

	dscp.SoftSolverConfig = config.BaseSolverConfigFromJson(solverConfig.SoftSolverConfig)
	dscp.AssignSolverConfig = config.BaseSolverConfigFromJson(solverConfig.AssignSolverConfig)
	dscp.UnassignSolverConfig = config.BaseSolverConfigFromJson(solverConfig.UnassignSolverConfig)
}
