package core

import (
	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
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

func (ms *MoveState) ToMoveStateJson() *smgjson.MoveStateJson {
	return &smgjson.MoveStateJson{
		ProposalId: ms.ProposalId,
		Signature:  ms.Signature,
		Actions:    ms.Actions,
		NextMove:   ms.CurrentAction,
	}
}

func MoveStateFromJson(msj *smgjson.MoveStateJson) *MoveState {
	return &MoveState{
		ProposalId:      msj.ProposalId,
		Signature:       msj.Signature,
		Actions:         msj.Actions,
		CurrentAction:   msj.NextMove,
		ActionConducted: 0,
	}
}

func (ms *MoveState) ApplyRemainingActions(snapshot *costfunc.Snapshot) {
	for i := ms.CurrentAction; i < len(ms.Actions); i++ {
		action := ms.Actions[i]
		// apply action to snapshot
		switch action.ActionType {
		// TODO
		default:
			ke := kerror.Create("UnsupportedAction", "unsupported action type="+string(action.ActionType))
			panic(ke)
		}
	}
}
