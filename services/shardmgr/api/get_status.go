package api

import (
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

type GetStateRequest struct {
	Favorite bool    `json:"favorite,omitempty"`
	Search   *string `json:"search,omitempty"`
}

type GetStateResponse struct {
	Workers []WorkerVm `json:"workers"`
	Shards  []ShardVm  `json:"shards"`
}

type ShardVm struct {
	ShardId  data.ShardId `json:"shard_id"`
	Replicas []*ReplicaVm `json:"replicas"`
}

type ReplicaVm struct {
	ReplicaIdx  data.ReplicaIdx     `json:"replica_idx"`
	Assignments []data.AssignmentId `json:"assignments,omitempty"`
}

type WorkerVm struct {
	WorkerFullId        string          `json:"worker_full_id"`
	WorkerId            string          `json:"worker_id"`
	SessionId           string          `json:"session_id,omitempty"`
	IsOffline           int8            `json:"is_offline"`
	IsShutdownReq       int8            `json:"is_shutdown_req,omitempty"`
	IsNotTarget         int8            `json:"is_not_target,omitempty"` // 1 if this worker is not available to move any shard into it
	IsDraining          int8            `json:"is_draining,omitempty"`
	IsShutdownPermitted int8            `json:"is_shutdown_permitted,omitempty"`
	WorkerStartTimeMs   int64           `json:"worker_start_time_ms,omitempty"`  // Unix timestamp in ms
	WorkerLastUpdateMs  int64           `json:"worker_last_update_ms,omitempty"` // Unix timestamp in ms
	Assignments         []*AssignmentVm `json:"assignments"`
}

type AssignmentVm struct {
	AssignmentId data.AssignmentId `json:"assignment_id"`
	ShardId      data.ShardId      `json:"shard_id"`
	ReplicaIdx   data.ReplicaIdx   `json:"replica_idx"`
	WorkerFullId string            `json:"worker_full_id"`
	Status       string            `json:"status,omitempty"` // e.g., "ready", "adding", "dropping", "dropped"
}
