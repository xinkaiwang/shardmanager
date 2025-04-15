package config

import "github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"

var (
	currentPathManager *PathManager
)

func GetCurrentPathManager() *PathManager {
	if currentPathManager == nil {
		currentPathManager = NewPathManager()
	}
	return currentPathManager
}

type PathManager struct {
}

func NewPathManager() *PathManager {
	return &PathManager{}
}

func (pm *PathManager) GetShardPlanPath() string {
	return "/smg/config/shard_plan.txt"
}

//	func (pm *PathManager) GetServiceInfoPath() string {
//		return "/smg/config/service_info.json"
//	}
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

func (pm *PathManager) GetPilotPathPrefix() string {
	return "/smg/pilot/"
}

func (pm *PathManager) GetMoveStatePrefix() string {
	return "/smg/move/"
}

func (pm *PathManager) GetRoutingPathPrefix() string {
	return "/smg/routing/"
}

func (pm *PathManager) FmtShardStatePath(shardId data.ShardId) string {
	return pm.GetShardStatePathPrefix() + string(shardId)
}

func (pm *PathManager) FmtWorkerStatePath(workerFullId data.WorkerFullId) string {
	// WorkerState node is using workerFullId as key (in case stateful_mem, it's workerId:sessionId. in case stateful_hd, it's just workerId).
	return pm.GetWorkerStatePathPrefix() + workerFullId.String()
}

func (pm *PathManager) FmtMoveStatePath(proposalId data.ProposalId) string {
	return pm.GetMoveStatePrefix() + string(proposalId)
}

func (pm *PathManager) FmtWorkerEphPath(workerFullId data.WorkerFullId) string {
	// Eph node is using workerId (not workerFullId) as key. This is to avoid multiple worker alive at same time.
	// The new worker should failed to start if the old worker is still alive.
	return pm.GetWorkerEphPathPrefix() + string(workerFullId.WorkerId)
}

func (pm *PathManager) FmtPilotPath(workerFullId data.WorkerFullId) string {
	// Pilot node is using workerId (not workerFullId) as key. Since it's not possible for multiple worker to alive at same time, it's safe.
	return pm.GetPilotPathPrefix() + string(workerFullId.WorkerId)
}

func (pm *PathManager) FmtRoutingPath(workerFullId data.WorkerFullId) string {
	// Routing node is using workerId (not workerFullId) as key.
	return pm.GetRoutingPathPrefix() + string(workerFullId.WorkerId)
}
