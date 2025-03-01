package smgjson

import (
	"encoding/json"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

type ProposalStateJson struct {
	ProposalId  string            `json:"proposal_id"`
	Description string            `json:"description"`
	Moves       []*MicroMovesJson `json:"moves"`
	NextMove    int               `json:"next_move"` // NextMove 是下一个要执行的 move 的索引
}

type MicroMovesJson struct {
	ShardId      data.ShardId      `json:"shard"`
	ReplicaIdx   data.ReplicaIdx   `json:"replica"`
	AssignmentId data.AssignmentId `json:"assignment"`
	From         data.WorkerFullId `json:"from"`
	To           data.WorkerFullId `json:"to"`
}

func ProposalStateJsonParse(data []byte) *ProposalStateJson {
	var state ProposalStateJson
	err := json.Unmarshal(data, &state)
	if err != nil {
		ke := kerror.Wrap(err, "UnmarshalError", "failed to unmarshal ProposalStateJson", false)
		panic(ke)
	}
	return &state
}

func (obj *ProposalStateJson) ToJson() string {
	bytes, err := json.Marshal(obj)
	if err != nil {
		ke := kerror.Wrap(err, "MarshalError", "failed to marshal ProposalStateJson", false)
		panic(ke)
	}
	return string(bytes)
}
