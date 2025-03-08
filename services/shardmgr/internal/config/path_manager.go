package config

import "github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"

type PathManager struct {
}

func NewPathManager() *PathManager {
	return &PathManager{}
}

func (pm *PathManager) GetShardPlanPath() string {
	return "/smg/config/shard_plan.txt"
}
func (pm *PathManager) GetServiceInfoPath() string {
	return "/smg/config/service_info.json"
}
func (pm *PathManager) GetServiceConfigPath() string {
	return "/smg/config/service_config.json"
}

func (pm *PathManager) GetShardStatePathPrefix() string {
	return "/smg/shard_state/"
}

func (pm *PathManager) GetWorkerStatePathPrefix() string {
	return "/smg/worker_state/"
}

func (pm *PathManager) GetWorkerEphPathPrefix() string {
	return "/smg/eph/"
}

func (pm *PathManager) GetExecutionPlanPrefix() string {
	return "/smg/move/"
}

func (pm *PathManager) FmtShardStatePath(shardId data.ShardId) string {
	return pm.GetShardStatePathPrefix() + string(shardId)
}

func (pm *PathManager) FmtWorkerStatePath(workerFullId data.WorkerFullId) string {
	return pm.GetWorkerStatePathPrefix() + workerFullId.String()
}

func (pm *PathManager) FmtExecutionPlanPath(proposalId data.ProposalId) string {
	return pm.GetExecutionPlanPrefix() + string(proposalId)
}
