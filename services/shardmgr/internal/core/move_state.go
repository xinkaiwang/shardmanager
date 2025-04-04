package core

import (
	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

type MoveState struct {
	ProposalId    data.ProposalId    `json:"proposal_id"`
	Signature     string             `json:"signature"`
	Actions       []*costfunc.Action `json:"actions"`
	CurrentAction int                `json:"current_action"` // CurrentAction is the index of the current action
}

func NewMoveStateFromProposal(ss *ServiceState, proposal *costfunc.Proposal) *MoveState {
	moveState := &MoveState{
		ProposalId:    proposal.ProposalId,
		Signature:     proposal.GetSignature(),
		Actions:       make([]*costfunc.Action, 0),
		CurrentAction: 0,
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
	}
	for _, action := range msj.Actions {
		moveState.Actions = append(moveState.Actions, costfunc.ActionFromJson(action))
	}
	return moveState
}

func (ms *MoveState) ApplyRemainingActions(snapshot *costfunc.Snapshot, mode costfunc.ApplyMode) {
	var action *costfunc.Action
	for i := ms.CurrentAction; i < len(ms.Actions); i++ {
		action = ms.Actions[i]
		// apply action to snapshot
		switch action.ActionType {
		case smgjson.AT_AddShard:
			ms.applyAddShardToSnapshot(snapshot, action, mode)
		case smgjson.AT_DropShard:
			ms.applyDropShardToSnapshot(snapshot, action, mode)
		case smgjson.AT_RemoveFromRoutingAndSleep, smgjson.AT_AddToRouting: // nothing to do
			continue
		default:
			ke := kerror.Create("UnsupportedAction", "unsupported action type="+string(action.ActionType))
			panic(ke)
		}
	}
}

func (ms *MoveState) applyAddShardToSnapshot(snapshot *costfunc.Snapshot, action *costfunc.Action, mode costfunc.ApplyMode) {
	// _, ok := snapshot.AllWorkers.Get(action.To)
	// if ok {
	// 	if mode == costfunc.AM_Strict {
	// 		ke := kerror.Create("ShardAlreadyExists", "shard already exists in snapshot")
	// 		panic(ke)
	// 	}
	// 	return // already exists
	// }
}

func (ms *MoveState) applyDropShardToSnapshot(snapshot *costfunc.Snapshot, action *costfunc.Action, mode costfunc.ApplyMode) {
	// TODO
	ke := kerror.Create("UnsupportedAction", "unsupported action type="+string(action.ActionType))
	panic(ke)
}
