package cougar

import (
	"github.com/xinkaiwang/shardmanager/services/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/services/unicorn/data"
)

type CougarAction string

const (
	CA_AddShard         CougarAction = "addShard"
	CA_RemoveShard      CougarAction = "removeShard"
	CA_UpdateShard      CougarAction = "updateShard"
	CA_ShutdownPermited CougarAction = "shutdownPermited"
)

type CougarShard struct {
	ShardId               data.ShardId
	ReplicaIdx            data.ReplicaIdx
	AssignmentId          data.AssignmentId
	CurrentConfirmedState cougarjson.CougarAssignmentState
	TargetState           cougarjson.CougarAssignmentState
	Properties            map[string]string
}

func NewCougarShard(shardId data.ShardId, replicaIdx data.ReplicaIdx, assignmentId data.AssignmentId) *CougarShard {
	return &CougarShard{
		ShardId:      shardId,
		ReplicaIdx:   replicaIdx,
		AssignmentId: assignmentId,
		Properties:   make(map[string]string),
	}
}

func (s *CougarShard) IncQueryCount(n int) {
	// TODO
}

type NotifyChangeFunc func(shardId data.ShardId, action CougarAction)

type CougarState struct {
	ShutdownPermited bool
	AllShards        map[data.ShardId]*CougarShard
}

func NewCougarStates() *CougarState {
	return &CougarState{
		AllShards: make(map[data.ShardId]*CougarShard),
	}
}

// return modify reason, empty means no modify
type CougarStateVisitor func(state *CougarState) string

type Cougar interface {
	VisitState(visitor CougarStateVisitor)
	RequestShutdown()
}
