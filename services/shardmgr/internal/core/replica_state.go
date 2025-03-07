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
	WorkerId   data.WorkerId
	LameDuck   bool

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

func (rs *ReplicaState) ToJson() *smgjson.ReplicaStateJson {
	obj := smgjson.NewReplicaStateJson()
	return obj
}

func (rs *ReplicaState) MarkAsSoftDelete(ctx context.Context) {
	rs.LameDuck = true
}
