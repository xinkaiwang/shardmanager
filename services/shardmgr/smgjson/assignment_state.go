package smgjson

import (
	"github.com/xinkaiwang/shardmanager/services/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

type AssignmentStateJson struct {
	ShardId      data.ShardId                     `json:"sid"`
	ReplicaIdx   data.ReplicaIdx                  `json:"rid,omitempty"`
	CurrentState cougarjson.CougarAssignmentState `json:"current_state,omitempty"`
	TargetState  cougarjson.CougarAssignmentState `json:"target_state,omitempty"`
	InRouting    int8                             `json:"in_routing,omitempty"`
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
