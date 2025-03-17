package core

import (
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

type AssignmentState struct {
	AssignmentId          data.AssignmentId
	ShardId               data.ShardId
	ReplicaIdx            data.ReplicaIdx
	WorkerFullId          data.WorkerFullId
	CurrentConfirmedState smgjson.AssignmentStateEnum
	TargetState           smgjson.AssignmentStateEnum
	ShouldInPilot         bool
	ShouldInRoutingTable  bool
}

func NewAssignmentState(assignmentId data.AssignmentId, shardId data.ShardId, replicaIdx data.ReplicaIdx, workerFullId data.WorkerFullId) *AssignmentState {
	return &AssignmentState{
		AssignmentId:          assignmentId,
		ShardId:               shardId,
		ReplicaIdx:            replicaIdx,
		WorkerFullId:          workerFullId,
		CurrentConfirmedState: smgjson.ASE_Unknown,
		TargetState:           smgjson.ASE_Unknown,
	}
}
