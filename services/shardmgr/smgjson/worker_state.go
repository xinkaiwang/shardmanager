package smgjson

import (
	"encoding/json"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

// path is "/smg/worker_state/{worker_full_id}"
type WorkerStateJson struct {
	// WorkerId
	WorkerId data.WorkerId `json:"worker_id"`

	SessionId data.SessionId `json:"session_id"`

	WorkerState data.WorkerStateEnum `json:"worker_state,omitempty"`

	Assignments map[data.AssignmentId]*AssignmentStateJson `json:"assignments"`

	LastUpdateAtMs   int64  `json:"update_time_ms,omitempty"`
	LastUpdateReason string `json:"update_reason,omitempty"`
}

func NewWorkerStateJson(workerId data.WorkerId, sessionId data.SessionId) *WorkerStateJson {
	return &WorkerStateJson{
		WorkerId:    workerId,
		SessionId:   sessionId,
		Assignments: make(map[data.AssignmentId]*AssignmentStateJson),
	}
}

func (obj *WorkerStateJson) SetUpdateReason(reason string) *WorkerStateJson {
	obj.LastUpdateAtMs = kcommon.GetWallTimeMs()
	obj.LastUpdateReason = reason
	return obj
}

func (obj *WorkerStateJson) ToJson() string {
	bytes, err := json.Marshal(obj)
	if err != nil {
		ke := kerror.Wrap(err, "MarshalError", "failed to marshal WorkerStateJson", false)
		panic(ke)
	}
	return string(bytes)
}

func WorkerStateJsonFromJson(stringJson string) *WorkerStateJson {
	var obj WorkerStateJson
	err := json.Unmarshal([]byte(stringJson), &obj)
	if err != nil {
		ke := kerror.Wrap(err, "UnmarshalError", "failed to unmarshal WorkerStateJson", false)
		panic(ke)
	}

	// Validate required fields
	if obj.WorkerId == "" {
		panic(kerror.Create("UnmarshalError", "missing required field: worker_id"))
	}
	if obj.SessionId == "" {
		panic(kerror.Create("UnmarshalError", "missing required field: session_id"))
	}
	if obj.Assignments == nil {
		obj.Assignments = make(map[data.AssignmentId]*AssignmentStateJson)
	}

	return &obj
}
