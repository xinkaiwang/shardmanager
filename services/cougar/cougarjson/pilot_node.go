package cougarjson

import (
	"encoding/json"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
)

// etcd path is "/smg/pilot/{worker_id}"
type PilotNodeJson struct {
	// WorkerId 是工作节点的唯一标识符, 通常是 hostname，也就是 pod name
	WorkerId string `json:"worker_id"`

	// SessionId unique ID to identity current process, changes in each restart/reboot.
	SessionId string `json:"session_id"`

	Assignments []*PilotAssignmentJson `json:"assignments"`

	LastUpdateAtMs   int64  `json:"update_time_ms,omitempty"`
	LastUpdateReason string `json:"update_reason,omitempty"`
}

func NewPilotNodeJson(workerId string, sessionId string, updateReason string) *PilotNodeJson {
	return &PilotNodeJson{
		WorkerId:         workerId,
		SessionId:        sessionId,
		Assignments:      make([]*PilotAssignmentJson, 0),
		LastUpdateAtMs:   kcommon.GetWallTimeMs(),
		LastUpdateReason: updateReason,
	}
}

// ToJson 将对象序列化为 JSON 字符串
func (obj *PilotNodeJson) ToJson() string {
	bytes, err := json.Marshal(obj)
	if err != nil {
		ke := kerror.Wrap(err, "MarshalError", "failed to marshal PilotNodeJson", false)
		panic(ke)
	}
	return string(bytes)
}

func ParsePilotNodeJson(stringJson string) *PilotNodeJson {
	var obj PilotNodeJson
	err := json.Unmarshal([]byte(stringJson), &obj)
	if err != nil {
		ke := kerror.Wrap(err, "UnmarshalError", "failed to unmarshal PilotNodeJson", false)
		panic(ke)
	}

	// 验证任务状态
	for _, assignment := range obj.Assignments {
		if !isValidPilotAssignmentState(assignment.State) {
			ke := kerror.Create("UnmarshalError", "invalid assignment state").
				With("state", string(assignment.State))
			panic(ke)
		}
	}

	return &obj
}

type PilotAssignmentJson struct {
	ShardId          string               `json:"shd"`
	ReplicaIdx       int                  `json:"idx,omitempty"`
	AsginmentId      string               `json:"asg"`
	CustomProperties map[string]string    `json:"custom_properties,omitempty"`
	State            PilotAssignmentState `json:"sts"`
}

// NewPilotAssignmentJson 创建一个新的 PilotAssignmentJson 实例
func NewPilotAssignmentJson(shardId string, replicaIdx int, assignmentId string, state PilotAssignmentState) *PilotAssignmentJson {
	if !isValidPilotAssignmentState(state) {
		ke := kerror.Create("InvalidState", "invalid assignment state").
			With("state", string(state))
		panic(ke)
	}

	return &PilotAssignmentJson{
		ShardId:          shardId,
		ReplicaIdx:       replicaIdx,
		AsginmentId:      assignmentId,
		CustomProperties: make(map[string]string),
		State:            state,
	}
}

// Define a custom type for State
type PilotAssignmentState string

// Define constants for each possible state
const (
	PAS_Unknown   PilotAssignmentState = "unknown"
	PAS_Active    PilotAssignmentState = "active"
	PAS_Completed PilotAssignmentState = "completed" // means this assignment needs to be removed.
)

// isValidPilotAssignmentState 检查任务状态是否有效
func isValidPilotAssignmentState(state PilotAssignmentState) bool {
	switch state {
	case PAS_Unknown, PAS_Active, PAS_Completed:
		return true
	default:
		return false
	}
} 