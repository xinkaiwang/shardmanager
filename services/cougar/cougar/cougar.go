package cougar

import "github.com/xinkaiwang/shardmanager/services/unicorn/data"

// type CougarAction string

// const (
// 	CA_AddShard    CougarAction = "addShard"
// 	CA_RemoveShard CougarAction = "removeShard"
// 	CA_UpdateShard CougarAction = "updateShard"
// )

type CougarShard struct {
	ShardId      data.ReplicaIdx
	ReplicaIdx   int
	AssignmentId data.AssignmentId
	Properties   map[string]string
}

func NewCougarShard(shardId data.ReplicaIdx, replicaIdx int, assignmentId data.AssignmentId) *CougarShard {
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

type NotifyChangeFunc func()

type ShardStates struct {
	ShutdownPermited bool
	AllShards        map[data.ShardId]*CougarShard
}

func NewCougarStates() *ShardStates {
	return &ShardStates{
		AllShards: make(map[data.ShardId]*CougarShard),
	}
}

type CougarStateVisitor func(state *ShardStates)

type Cougar interface {
	VisitState(visitor CougarStateVisitor)
	RequestShutdown()
}
