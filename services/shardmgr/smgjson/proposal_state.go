package smgjson

import (
	"encoding/json"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
)

type ProposalStateJson struct {
	ProposalId string           `json:"proposal_id"`
	Moves      []*MoveStateJson `json:"moves"`
}

type MoveStateJson struct {
	MoveIdx       int    `json:"move_idx"`
	ReplicaFullId string `json:"replica"`
	From          string `json:"from"`
	To            string `json:"to"`
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
