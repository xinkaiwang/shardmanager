package unicornjson

import (
	"encoding/json"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
)

// key="/smg/routing/{worker_id}"
type WorkerEntryJson struct {
	// WorkerId 是工作节点的唯一标识符
	WorkerId string `json:"worker_id"`

	AddressPort string `json:"addr_port"` // for exp: "10.0.0.32:8080"

	// Capacity 表示工作节点的处理容量
	Capacity int32 `json:"capacity"`

	Assignments []*AssignmentJson `json:"assignments,omitempty"` // assignments is always sorted by shardId

	LastUpdateAtMs   int64  `json:"update_time_ms,omitempty"`
	LastUpdateReason string `json:"update_reason,omitempty"`
}

func NewWorkerEntryJson(workerId string, addressPort string, capacity int32, updateReason string) *WorkerEntryJson {
	return &WorkerEntryJson{
		WorkerId:         workerId,
		AddressPort:      addressPort,
		Capacity:         capacity,
		Assignments:      make([]*AssignmentJson, 0),
		LastUpdateAtMs:   kcommon.GetWallTimeMs(),
		LastUpdateReason: updateReason,
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

func (a *WorkerEntryJson) EqualsTo(b *WorkerEntryJson) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.WorkerId != b.WorkerId {
		return false
	}
	if a.AddressPort != b.AddressPort {
		return false
	}
	if a.Capacity != b.Capacity {
		return false
	}
	if len(a.Assignments) != len(b.Assignments) {
		return false
	}
	for i, assignA := range a.Assignments {
		if !assignA.EqualsTo(b.Assignments[i]) {
			return false
		}
	}
	// last update time is not compared
	return true
}

func (a *AssignmentJson) EqualsTo(b *AssignmentJson) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.ShardId != b.ShardId {
		return false
	}
	if a.ReplicaIdx != b.ReplicaIdx {
		return false
	}
	if a.AsginmentId != b.AsginmentId {
		return false
	}
	return true
}
