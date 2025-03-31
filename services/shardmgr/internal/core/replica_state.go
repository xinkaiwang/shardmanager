package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

type ReplicaState struct {
	ShardId    data.ShardId
	ReplicaIdx data.ReplicaIdx
	LameDuck   bool

	Assignments map[data.AssignmentId]common.Unit
}

func NewReplicaState(shardId data.ShardId, replicaIdx data.ReplicaIdx) *ReplicaState {
	return &ReplicaState{
		ShardId:     shardId,
		ReplicaIdx:  replicaIdx,
		Assignments: make(map[data.AssignmentId]common.Unit),
	}
}

func (rs *ReplicaState) ToJson() *smgjson.ReplicaStateJson {
	obj := smgjson.NewReplicaStateJson()
	if rs.ReplicaIdx != 0 {
		idx := int32(rs.ReplicaIdx)
		obj.ReplicaIdx = &idx
	}
	for assignId := range rs.Assignments {
		obj.Assignments = append(obj.Assignments, string(assignId))
	}
	return obj
}

func (rs *ReplicaState) MarkAsSoftDelete(ctx context.Context) {
	rs.LameDuck = true
}
