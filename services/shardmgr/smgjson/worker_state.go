package smgjson

import (
	"encoding/json"

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

	LastUpdateAtMs   int64             `json:"update_time_ms,omitempty"`
	LastUpdateReason string            `json:"update_reason,omitempty"`
	RequestShutdown  int8              `json:"req_shutdown,omitempty"`
	StatefulType     data.StatefulType `json:"stateful_type,omitempty"` // stateless, ST_MEMORY, ST_HARD_DRIVE
}

func NewWorkerStateJson(workerId data.WorkerId, sessionId data.SessionId, statefulType data.StatefulType) *WorkerStateJson {
	return &WorkerStateJson{
		WorkerId:     workerId,
		SessionId:    sessionId,
		Assignments:  make(map[data.AssignmentId]*AssignmentStateJson),
		StatefulType: statefulType,
	}
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

// Equals 比较两个 WorkerStateJson 是否相等
func (obj *WorkerStateJson) Equals(other *WorkerStateJson) bool {
	if obj == nil && other == nil {
		return true
	}
	if obj == nil || other == nil {
		return false
	}

	// 比较基本字段
	if obj.WorkerId != other.WorkerId ||
		obj.SessionId != other.SessionId ||
		obj.WorkerState != other.WorkerState ||
		obj.LastUpdateAtMs != other.LastUpdateAtMs ||
		obj.LastUpdateReason != other.LastUpdateReason {
		return false
	}

	// 比较 map 大小
	if len(obj.Assignments) != len(other.Assignments) {
		return false
	}

	// 比较 map 内容
	for key, thisVal := range obj.Assignments {
		otherVal, exists := other.Assignments[key]
		if !exists {
			return false
		}

		// 比较指针值，如果都为 nil 则相等
		if thisVal == nil && otherVal == nil {
			continue
		}
		if thisVal == nil || otherVal == nil {
			return false
		}

		// 这里假设 AssignmentStateJson 可以直接用 == 比较
		// 如果 AssignmentStateJson 也包含 map，需要为它也实现 Equals 方法并在这里调用
		if *thisVal != *otherVal {
			return false
		}
	}

	return true
}
