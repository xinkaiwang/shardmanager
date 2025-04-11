package cougar

import (
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/services/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/services/unicorn/data"
)

type PilotNode struct {
	// WorkerId 是工作节点的唯一标识符, 通常是 hostname，也就是 pod name
	WorkerId data.WorkerId `json:"worker_id"`

	// SessionId unique ID to identity current process, changes in each restart/reboot.
	SessionId data.SessionId `json:"session_id"`

	Assignments map[data.AssignmentId]*PilotAssignment `json:"assignments"`

	ShutdownPermited bool   `json:"shutdown_permited,omitempty"`
	LastUpdateAtMs   int64  `json:"update_time_ms,omitempty"`
	LastUpdateReason string `json:"update_reason,omitempty"`
}

func NewPilotNode(workerId data.WorkerId, sessionId data.SessionId, updateReason string) *PilotNode {
	return &PilotNode{
		WorkerId:         workerId,
		SessionId:        sessionId,
		Assignments:      make(map[data.AssignmentId]*PilotAssignment),
		LastUpdateAtMs:   kcommon.GetWallTimeMs(),
		LastUpdateReason: updateReason,
	}
}

type PilotAssignment struct {
	ShardId     data.ShardId                     `json:"shd"`
	ReplicaIdx  data.ReplicaIdx                  `json:"idx,omitempty"`
	AsginmentId data.AssignmentId                `json:"asg"`
	State       cougarjson.CougarAssignmentState `json:"state"`
}

func PilotNodeFromJson(node *cougarjson.PilotNodeJson) *PilotNode {
	pilotNode := NewPilotNode(data.WorkerId(node.WorkerId), data.SessionId(node.SessionId), node.LastUpdateReason)
	pilotNode.ShutdownPermited = node.ShutdownPermited == 1
	pilotNode.LastUpdateAtMs = node.LastUpdateAtMs

	for _, assignment := range node.Assignments {
		pilotNode.Assignments[data.AssignmentId(assignment.AsginmentId)] = &PilotAssignment{
			ShardId:     data.ShardId(assignment.ShardId),
			ReplicaIdx:  data.ReplicaIdx(assignment.ReplicaIdx),
			AsginmentId: data.AssignmentId(assignment.AsginmentId),
			State:       assignment.State,
		}
	}

	return pilotNode
}
