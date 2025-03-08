package smgjson

import (
	"encoding/json"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

// <etcd>/smg/move/{proposal_id}
type ExecutionPlanJson struct {
	ProposalId data.ProposalId   `json:"proposal_id"`
	Signature  string            `json:"signature"`
	Moves      []*MicroMovesJson `json:"moves"`
	NextMove   int               `json:"next_move"` // NextMove 是下一个要执行的 move 的索引
}

type MicroMovesJson struct {
	ShardId      data.ShardId      `json:"shard"`
	ReplicaIdx   data.ReplicaIdx   `json:"replica,omitempty"`
	AssignmentId data.AssignmentId `json:"assignment"`
	From         string            `json:"from,omitempty"` // WorkerFullId
	To           string            `json:"to,omitempty"`   // WorkerFullId
}

func ExecutionPlanJsonParse(data []byte) *ExecutionPlanJson {
	var state ExecutionPlanJson
	err := json.Unmarshal(data, &state)
	if err != nil {
		ke := kerror.Wrap(err, "UnmarshalError", "failed to unmarshal ProposalStateJson", false)
		panic(ke)
	}
	return &state
}

func (obj *ExecutionPlanJson) ToJson() string {
	bytes, err := json.Marshal(obj)
	if err != nil {
		ke := kerror.Wrap(err, "MarshalError", "failed to marshal ProposalStateJson", false)
		panic(ke)
	}
	return string(bytes)
}
