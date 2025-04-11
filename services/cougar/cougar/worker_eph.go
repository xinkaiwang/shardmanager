package cougar

import (
	"github.com/xinkaiwang/shardmanager/services/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/services/unicorn/data"
)

type WorkerEphNode struct {
	// WorkerId 是工作节点的唯一标识符, 通常是 hostname，也就是 pod name
	WorkerId data.WorkerId

	// SessionId unique ID to identity current process, changes in each restart/reboot.
	SessionId data.SessionId

	AddressPort string // for exp: "10.0.0.32:8080"

	// StartTimeMs 是工作节点启动的时间戳（毫秒）
	StartTimeMs int64

	// Capacity 表示工作节点的处理容量 (100 means 100%)
	Capacity int32

	MemorySizeMB int32 // how many memory (or vmem, depend on which resource is on the critical path) in this worker

	// Properties 存储工作节点的额外属性
	// 键和值都是字符串类型
	Properties map[string]string // gpu_ct="1", gpu_type="H100", etc.

	Assignments map[data.AssignmentId]*Assignment

	LastUpdateAtMs   int64
	LastUpdateReason string

	ReqShutDown bool // if true, worker should drain and shutdown
}

type Assignment struct {
	ShardId     string                           `json:"shd"`
	ReplicaIdx  int                              `json:"idx,omitempty"`
	AsginmentId string                           `json:"asg"`
	State       cougarjson.CougarAssignmentState `json:"sts"`
}

func NewWorkerEphNode(workerId data.WorkerId, sessionId data.SessionId, startTimeMs int64, capacity int32) *WorkerEphNode {
	return &WorkerEphNode{
		WorkerId:         workerId,
		SessionId:        sessionId,
		StartTimeMs:      startTimeMs,
		Capacity:         capacity,
		Properties:       make(map[string]string),
		Assignments:      make(map[data.AssignmentId]*Assignment),
		LastUpdateAtMs:   startTimeMs,
		LastUpdateReason: "init",
	}
}

func (ephNode *WorkerEphNode) ToJson() *cougarjson.WorkerEphJson {
	wej := &cougarjson.WorkerEphJson{
		WorkerId:         string(ephNode.WorkerId),
		SessionId:        string(ephNode.SessionId),
		StartTimeMs:      ephNode.StartTimeMs,
		Capacity:         ephNode.Capacity,
		MemorySizeMB:     ephNode.MemorySizeMB,
		LastUpdateAtMs:   ephNode.LastUpdateAtMs,
		LastUpdateReason: ephNode.LastUpdateReason,
		ReqShutDown:      0,
	}

	if ephNode.ReqShutDown {
		wej.ReqShutDown = 1
	}

	for _, assignment := range ephNode.Assignments {
		asg := &cougarjson.AssignmentJson{
			ShardId:     assignment.ShardId,
			ReplicaIdx:  assignment.ReplicaIdx,
			AsginmentId: assignment.AsginmentId,
			State:       assignment.State,
		}
		wej.Assignments = append(wej.Assignments, asg)
	}

	return wej
}
