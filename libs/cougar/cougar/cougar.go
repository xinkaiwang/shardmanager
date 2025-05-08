package cougar

import (
	"github.com/xinkaiwang/shardmanager/libs/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/libs/unicorn/data"
)

type CougarAction string

const (
	CA_AddShard         CougarAction = "addShard"
	CA_RemoveShard      CougarAction = "removeShard"
	CA_UpdateShard      CougarAction = "updateShard"
	CA_ShutdownPermited CougarAction = "shutdownPermited"
)

type ShardInfo struct {
	ShardId               data.ShardId
	ReplicaIdx            data.ReplicaIdx
	AssignmentId          data.AssignmentId
	CurrentConfirmedState cougarjson.CougarAssignmentState
	TargetState           cougarjson.CougarAssignmentState
	Properties            map[string]string
	AppShard              AppShard
	ChReady               chan struct{} // notify when shard is ready
	ChDropped             chan struct{} // notify when shard is dropped

	stats *cougarjson.ShardStats
}

func NewCougarShardInfo(shardId data.ShardId, replicaIdx data.ReplicaIdx, assignmentId data.AssignmentId) *ShardInfo {
	return &ShardInfo{
		ShardId:      shardId,
		ReplicaIdx:   replicaIdx,
		AssignmentId: assignmentId,
		Properties:   make(map[string]string),
		ChReady:      make(chan struct{}),
		ChDropped:    make(chan struct{}),
	}
}

func (s *ShardInfo) ReportQueryCount(n int64) {
	// TODO
}

func (s *ShardInfo) IsReady() bool {
	select {
	case <-s.ChReady:
		return true
	default:
		return false
	}
}

func (s *ShardInfo) IsDropped() bool {
	select {
	case <-s.ChDropped:
		return true
	default:
		return false
	}
}

type NotifyChangeFunc func(shardId data.ShardId, action CougarAction)

type CougarState struct {
	ShutDownRequest  chan struct{}
	ShutdownPermited chan struct{}
	AllShards        map[data.ShardId]*ShardInfo
}

func NewCougarStates() *CougarState {
	return &CougarState{
		ShutDownRequest:  make(chan struct{}),
		ShutdownPermited: make(chan struct{}),
		AllShards:        make(map[data.ShardId]*ShardInfo),
	}
}

func (s *CougarState) IsShutdownRequested() bool {
	select {
	case <-s.ShutDownRequest:
		return true
	default:
		return false
	}
}

func (s *CougarState) IsShutdownPermited() bool {
	select {
	case <-s.ShutdownPermited:
		return true
	default:
		return false
	}
}

// return modify reason, empty means no modify
type CougarStateVisitor func(state *CougarState) string

type Cougar interface {
	VisitState(visitor CougarStateVisitor)
	RequestShutdown() chan struct{} // the channel will be closed when shutdown is permitted
}
