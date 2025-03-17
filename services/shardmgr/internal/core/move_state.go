package core

import (
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

type MoveState struct {
	ProposalId      data.ProposalId       `json:"proposal_id"`
	Signature       string                `json:"signature"`
	Actions         []*smgjson.ActionJson `json:"actions"`
	CurrentAction   int                   `json:"current_action"`   // CurrentAction is the index of the current action
	ActionConducted int8                  `json:"action_conducted"` // true means current action is already sent out (or conducted)
}

func NewMoveStateFromProposal(ss *ServiceState, proposal *costfunc.Proposal) *MoveState {
	moveState := &MoveState{
		ProposalId:      proposal.ProposalId,
		Signature:       proposal.GetSignature(),
		Actions:         make([]*smgjson.ActionJson, 0),
		CurrentAction:   0,
		ActionConducted: 0,
	}
	moveState.Actions = proposal.Move.GetActions(ss.ServiceConfig.ShardConfig)
	return moveState
}
