package core

import (
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

type MoveState struct {
	ProposalId    data.ProposalId    `json:"proposal_id"`
	Signature     string             `json:"signature"`
	Actions       []*costfunc.Action `json:"actions"`
	CurrentAction int                `json:"current_action"` // CurrentAction is the index of the current action
	SolverType    string             `json:"solver_type"`    // SolverType is the type of the solver that proposed this move
}

func NewMoveStateFromProposal(ss *ServiceState, proposal *costfunc.Proposal) *MoveState {
	moveState := &MoveState{
		ProposalId:    proposal.ProposalId,
		Signature:     proposal.GetSignature(),
		Actions:       make([]*costfunc.Action, 0),
		CurrentAction: 0,
		SolverType:    proposal.SolverType,
	}
	moveState.Actions = proposal.Move.GetActions(ss.ServiceConfig.ShardConfig)
	return moveState
}

func (ms *MoveState) ToMoveStateJson(updateReason string) *smgjson.MoveStateJson {
	moveStateJson := &smgjson.MoveStateJson{
		ProposalId:   ms.ProposalId,
		Signature:    ms.Signature,
		NextMove:     ms.CurrentAction,
		UpdateReason: updateReason,
		SolverType:   ms.SolverType,
	}
	for _, action := range ms.Actions {
		moveStateJson.Actions = append(moveStateJson.Actions, action.ToJson())
	}
	return moveStateJson
}

func MoveStateFromJson(msj *smgjson.MoveStateJson) *MoveState {
	moveState := &MoveState{
		ProposalId:    msj.ProposalId,
		Signature:     msj.Signature,
		CurrentAction: msj.NextMove,
		SolverType:    msj.SolverType,
	}
	for _, action := range msj.Actions {
		moveState.Actions = append(moveState.Actions, costfunc.ActionFromJson(action))
	}
	return moveState
}

func (ms *MoveState) ApplyRemainingActions(snapshot *costfunc.Snapshot, mode costfunc.ApplyMode) {
	for i := ms.CurrentAction; i < len(ms.Actions); i++ {
		ms.Actions[i].ApplyToSnapshot(snapshot, mode)
	}
}
