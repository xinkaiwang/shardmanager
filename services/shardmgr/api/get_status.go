package api

import (
	"github.com/xinkaiwang/shardmanager/libs/cougar/cougarjson"
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
	WorkerFullId string          `json:"worker_full_id"`
	Assignments  []*AssignmentVm `json:"assignments,omitempty"`
}

type AssignmentVm struct {
	AssignmentId data.AssignmentId                `json:"assignment_id"`
	ShardId      data.ShardId                     `json:"shard_id"`
	ReplicaIdx   data.ReplicaIdx                  `json:"replica_idx"`
	WorkerFullId string                           `json:"worker_full_id"`
	CurrentState cougarjson.CougarAssignmentState `json:"current_state"`
	TargetState  cougarjson.CougarAssignmentState `json:"target_state"`
}
