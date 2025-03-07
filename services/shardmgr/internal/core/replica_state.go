package core

import (
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

type ReplicaState struct {
	ShardId    data.ShardId
	ReplicaIdx data.ReplicaIdx
	WorkerId   data.WorkerId

	Assignments map[data.AssignmentId]common.Unit
}

func NewReplicaState(shardId data.ShardId, replicaIdx data.ReplicaIdx, workerId data.WorkerId) *ReplicaState {
	return &ReplicaState{
		ShardId:     shardId,
		ReplicaIdx:  replicaIdx,
		WorkerId:    workerId,
		Assignments: make(map[data.AssignmentId]common.Unit),
	}
}
