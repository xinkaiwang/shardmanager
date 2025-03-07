package smgjson

import "github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"

type AssignmentStateJson struct {
	ShardId    data.ShardId    `json:"sid"`
	ReplicaIdx data.ReplicaIdx `json:"rid,omitempty"`
}

func NewAssignmentStateJson(shardId data.ShardId, replicaIdx data.ReplicaIdx) *AssignmentStateJson {
	return &AssignmentStateJson{
		ShardId:    shardId,
		ReplicaIdx: replicaIdx,
	}
}
