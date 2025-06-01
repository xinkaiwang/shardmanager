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

func (as *AssignmentState) GetStateVmString() string {
	switch as.TargetState {
	case cougarjson.CAS_Ready:
		if as.CurrentConfirmedState == cougarjson.CAS_Ready {
			return "ready"
		} else if as.CurrentConfirmedState == cougarjson.CAS_Unknown {
			return "adding"
		} else {
			return "invalid state: current=" + string(as.CurrentConfirmedState) + ", target=" + string(as.TargetState)
		}
	case cougarjson.CAS_Dropped:
		if as.CurrentConfirmedState == cougarjson.CAS_Dropped {
			return "dropped"
		} else if as.CurrentConfirmedState == cougarjson.CAS_Unknown || as.CurrentConfirmedState == cougarjson.CAS_Ready || as.CurrentConfirmedState == cougarjson.CAS_Dropping {
			return "dropping"
		} else {
			return "invalid state: current=" + string(as.CurrentConfirmedState) + ", target=" + string(as.TargetState)
		}
	default:
		return "invalid state: current=" + string(as.CurrentConfirmedState) + ", target=" + string(as.TargetState)
	}
}
