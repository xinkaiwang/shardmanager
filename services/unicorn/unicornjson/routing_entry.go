package unicornjson

import (
	"encoding/json"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
)

// key="/smg/routing/{worker_id}"
type WorkerEntryJson struct {
	// WorkerId 是工作节点的唯一标识符
	WorkerId string `json:"worker_id"`

	AddressPort string `json:"addr_port"` // for exp: "10.0.0.32:8080"

	// Capacity 表示工作节点的处理容量
	Capacity int32 `json:"capacity"`

	Assignments []*AssignmentJson `json:"assignments,omitempty"`
}

func NewWorkerEntryJson(workerId string, addressPort string, capacity int32) *WorkerEntryJson {
	return &WorkerEntryJson{
		WorkerId:    workerId,
		AddressPort: addressPort,
		Capacity:    capacity,
		Assignments: make([]*AssignmentJson, 0),
	}
}

type AssignmentJson struct {
	ShardId     string `json:"shd"`
	ReplicaIdx  int    `json:"idx,omitempty"`
	AsginmentId string `json:"asg"`
}

func NewAssignmentJson(shardId string, replicaIdx int, assignmentId string) *AssignmentJson {
	return &AssignmentJson{
		ShardId:     shardId,
		ReplicaIdx:  replicaIdx,
		AsginmentId: assignmentId,
	}
}

// return json string
func (obj *WorkerEntryJson) ToJson() string {
	bytes, err := json.Marshal(obj)
	if err != nil {
		ke := kerror.Wrap(err, "MarshalError", "failed to marshal WorkerEntryJson", false)
		panic(ke)
	}
	return string(bytes)
}

func WorkerEntryJsonFromJson(stringJson string) *WorkerEntryJson {
	var obj WorkerEntryJson
	err := json.Unmarshal([]byte(stringJson), &obj)
	if err != nil {
		ke := kerror.Wrap(err, "UnmarshalError", "failed to unmarshal WorkerEntryJson", false)
		panic(ke)
	}
	return &obj
}
