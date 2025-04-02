package cougarjson

import (
	"encoding/json"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
)

// etcd path is "/smg/eph/{worker_id}:{session_id}"
type WorkerEphJson struct {
	// WorkerId 是工作节点的唯一标识符, 通常是 hostname，也就是 pod name
	WorkerId string `json:"worker_id"`

	// SessionId unique ID to identity current process, changes in each restart/reboot.
	SessionId string `json:"session_id"`

	AddressPort string `json:"addr_port,omitempty"` // for exp: "10.0.0.32:8080"

	// StartTimeMs 是工作节点启动的时间戳（毫秒）
	StartTimeMs int64 `json:"start_time_ms"`

	// Capacity 表示工作节点的处理容量 (100 means 100%)
	Capacity int32 `json:"capacity"`

	MemorySizeMB int32 `json:"memory_size_mb,omitempty"` // how many memory (or vmem, depend on which resource is on the critical path) in this worker

	// Properties 存储工作节点的额外属性
	// 键和值都是字符串类型
	Properties map[string]string `json:"properties,omitempty"` // gpu_ct="1", gpu_type="H100", etc.

	Assignments []*AssignmentJson `json:"assignments,omitempty"`

	LastUpdateAtMs   int64  `json:"update_time_ms,omitempty"`
	LastUpdateReason string `json:"update_reason,omitempty"`

	ReqShutDown int8 `json:"req_shutdown,omitempty"` // if true, worker should drain and shutdown
}

func NewWorkerEphJson(workerId string, sessionId string, startTimeMs int64, capacity int32) *WorkerEphJson {
	return &WorkerEphJson{
		WorkerId:    workerId,
		SessionId:   sessionId,
		StartTimeMs: startTimeMs,
		Capacity:    capacity,
		Properties:  make(map[string]string),
		Assignments: make([]*AssignmentJson, 0),
	}
}

// return json string
func (obj *WorkerEphJson) ToJson() string {
	bytes, err := json.Marshal(obj)
	if err != nil {
		ke := kerror.Wrap(err, "MarshalError", "failed to marshal AssignmentJson", false)
		panic(ke)
	}
	return string(bytes)
}

func WorkerEphJsonFromJson(stringJson string) *WorkerEphJson {
	var obj WorkerEphJson
	err := json.Unmarshal([]byte(stringJson), &obj)
	if err != nil {
		ke := kerror.Wrap(err, "UnmarshalError", "failed to unmarshal WorkerEphJson", false)
		panic(ke)
	}
	return &obj
}

// Define a custom type for State
type CougarAssignmentState string

// Define constants for each possible state
const (
	CAS_Unknown  CougarAssignmentState = "unkonwn"
	CAS_Adding   CougarAssignmentState = "adding"
	CAS_Ready    CougarAssignmentState = "ready"
	CAS_Dropping CougarAssignmentState = "dropping"
	CAS_Dropped  CougarAssignmentState = "dropped"
)

// Update the ReplicaJson struct to use the custom State type
type AssignmentJson struct {
	ShardId     string                `json:"shd"`
	ReplicaIdx  int                   `json:"idx,omitempty"`
	AsginmentId string                `json:"asg"`
	State       CougarAssignmentState `json:"sts"`
}

func NewAssignmentJson(shardId string, replicaIdx int, assignmentId string, state CougarAssignmentState) *AssignmentJson {
	return &AssignmentJson{
		ShardId:     shardId,
		ReplicaIdx:  replicaIdx,
		AsginmentId: assignmentId,
		State:       state,
	}
}

// return json string
func (obj *AssignmentJson) ToJson() string {
	bytes, err := json.Marshal(obj)
	if err != nil {
		ke := kerror.Wrap(err, "MarshalError", "failed to marshal AssignmentJson", false)
		panic(ke)
	}
	return string(bytes)
}
