package core

import (
	"encoding/json"

	"github.com/xinkaiwang/shardmanager/libs/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

type AssignmentState struct {
	AssignmentId          data.AssignmentId
	ShardId               data.ShardId
	ReplicaIdx            data.ReplicaIdx
	WorkerFullId          data.WorkerFullId
	CurrentConfirmedState cougarjson.CougarAssignmentState
	TargetState           cougarjson.CougarAssignmentState
	ShouldInPilot         bool
	ShouldInRoutingTable  bool
}

func NewAssignmentState(assignmentId data.AssignmentId, shardId data.ShardId, replicaIdx data.ReplicaIdx, workerFullId data.WorkerFullId) *AssignmentState {
	return &AssignmentState{
		AssignmentId:          assignmentId,
		ShardId:               shardId,
		ReplicaIdx:            replicaIdx,
		WorkerFullId:          workerFullId,
		CurrentConfirmedState: cougarjson.CAS_Unknown,
		TargetState:           cougarjson.CAS_Unknown,
	}
}

func (as *AssignmentState) String() string {
	data, err := json.Marshal(as)
	if err != nil {
		ke := kerror.Wrap(err, "MarshalError", "failed to marshal AssignmentState", false)
		panic(ke)
	}
	return string(data)
}
