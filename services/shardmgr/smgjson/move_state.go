package smgjson

import (
	"encoding/json"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

// <etcd>/smg/move/{proposal_id}
type MoveStateJson struct {
	ProposalId   data.ProposalId `json:"proposal_id"`
	Signature    string          `json:"signature"`
	Actions      []*ActionJson   `json:"moves"`
	NextMove     int             `json:"next_move"`     // NextMove 是下一个要执行的 move 的索引
	UpdateReason string          `json:"update_reason"` // UpdateReason 更新原因
}

type ActionJson struct {
	ActionType           ActionType        `json:"name,omitempty"`
	ShardId              data.ShardId      `json:"shard,omitempty"`
	ReplicaIdx           data.ReplicaIdx   `json:"replica,omitempty"`
	AssignmentId         data.AssignmentId `json:"assignment,omitempty"`
	From                 string            `json:"from,omitempty"` // WorkerFullId
	To                   string            `json:"to,omitempty"`   // WorkerFullId
	RemoveSrcFromRouting int8              `json:"remove_src_from_routing,omitempty"`
	AddDestToRouting     int8              `json:"add_dest_to_routing,omitempty"`
	SleepMs              int               `json:"sleep_ms,omitempty"`
	Stage                ActionStage       `json:"stage,omitempty"` //
}

type ActionStage string

const (
	AS_NotStarted ActionStage = ""
	AS_Conducted  ActionStage = "conducted"
	AS_Completed  ActionStage = "completed"
)

func MoveStateJsonParse(data string) *MoveStateJson {
	var state MoveStateJson
	err := json.Unmarshal([]byte(data), &state)
	if err != nil {
		ke := kerror.Wrap(err, "UnmarshalError", "failed to unmarshal MoveStateJson", false)
		panic(ke)
	}
	return &state
}

func (obj *MoveStateJson) ToJson() string {
	bytes, err := json.Marshal(obj)
	if err != nil {
		ke := kerror.Wrap(err, "MarshalError", "failed to marshal MoveStateJson", false)
		panic(ke)
	}
	return string(bytes)
}

func (obj *MoveStateJson) WithUpdateReason(reason string) *MoveStateJson {
	obj.UpdateReason = reason
	return obj
}

type ActionType string

const (
	AT_RemoveFromRoutingAndSleep ActionType = "remove_from_routing"
	AT_AddToRouting              ActionType = "add_to_routing"
	AT_DropShard                 ActionType = "move_shard"
	AT_AddShard                  ActionType = "add_shard"
)
