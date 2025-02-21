package core

type PathManager struct {
}

func NewPathManager() *PathManager {
	return &PathManager{}
}

func (pm *PathManager) GetShardPlanPath() string {
	return "/smg/config/shard_plan.txt"
}

func (pm *PathManager) GetShardStatePathPrefix() string {
	return "/smg/shard_state/"
}

func (pm *PathManager) GetWorkerStatePathPrefix() string {
	return "/smg/worker_state/"
}

func (pm *PathManager) GetWorkerEphPathPrefix() string {
	return "/smg/discover/"
}
