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

	Assignments []*PilotAssignmentJson `json:"asgs"`

	ShutdownPermited int8   `json:"shutdown_permited,omitempty"`
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

	return &obj
}

type PilotAssignmentJson struct {
	ShardId          string                `json:"shd"`
	ReplicaIdx       int                   `json:"idx,omitempty"`
	AssignmentId     string                `json:"asg"`
	CustomProperties map[string]string     `json:"data,omitempty"`
	State            CougarAssignmentState `json:"sts"`
}

// NewPilotAssignmentJson 创建一个新的 PilotAssignmentJson 实例
func NewPilotAssignmentJson(shardId string, replicaIdx int, assignmentId string, state CougarAssignmentState) *PilotAssignmentJson {
	return &PilotAssignmentJson{
		ShardId:          shardId,
		ReplicaIdx:       replicaIdx,
		AssignmentId:     assignmentId,
		CustomProperties: make(map[string]string),
		State:            state,
	}
}

func (obj *PilotNodeJson) EqualsTo(other *PilotNodeJson) bool {
	if obj.WorkerId != other.WorkerId {
		return false
	}
	if obj.SessionId != other.SessionId {
		return false
	}
	if len(obj.Assignments) != len(other.Assignments) {
		return false
	}
	for i, assignment := range obj.Assignments {
		if !assignment.EqualsTo(other.Assignments[i]) {
			return false
		}
	}
	if obj.ShutdownPermited != other.ShutdownPermited {
		return false
	}
	if obj.LastUpdateAtMs != other.LastUpdateAtMs {
		return false
	}
	if obj.LastUpdateReason != other.LastUpdateReason {
		return false
	}
	return true
}

func (obj *PilotAssignmentJson) EqualsTo(other *PilotAssignmentJson) bool {
	if obj.ShardId != other.ShardId {
		return false
	}
	if obj.ReplicaIdx != other.ReplicaIdx {
		return false
	}
	if obj.AssignmentId != other.AssignmentId {
		return false
	}
	if obj.State != other.State {
		return false
	}
	return true
}

func (obj *PilotAssignmentJson) ToJson() string {
	bytes, err := json.Marshal(obj)
	if err != nil {
		ke := kerror.Wrap(err, "MarshalError", "failed to marshal PilotAssignmentJson", false)
		panic(ke)
	}
	return string(bytes)
}

// // Define a custom type for State
// type PilotAssignmentState string

// // Define constants for each possible state
// const (
// 	PAS_Unknown   PilotAssignmentState = "unknown"
// 	PAS_Active    PilotAssignmentState = "active"
// 	PAS_Completed PilotAssignmentState = "completed" // means this assignment needs to be removed.
// )

// // isValidPilotAssignmentState 检查任务状态是否有效
// func isValidPilotAssignmentState(state PilotAssignmentState) bool {
// 	switch state {
// 	case PAS_Unknown, PAS_Active, PAS_Completed:
// 		return true
// 	default:
// 		return false
// 	}
// }
