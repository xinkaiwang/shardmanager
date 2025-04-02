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

// type AssignmentStateEnum string

// const (
// 	ASE_Unknown AssignmentStateEnum = "unknown"
// 	ASE_Healthy AssignmentStateEnum = "healthy"
// 	ASE_Dropped AssignmentStateEnum = "dropped"
// )
