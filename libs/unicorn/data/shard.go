package data

import "strconv"

type ShardId string

type ReplicaIdx int

type AssignmentId string

type ReplicaFullId struct {
	ShardId    ShardId
	ReplicaIdx ReplicaIdx
}

func NewReplicaFullId(shardId ShardId, replicaIdx ReplicaIdx) ReplicaFullId {
	return ReplicaFullId{
		ShardId:    shardId,
		ReplicaIdx: replicaIdx,
	}
}

func (replicaFullId ReplicaFullId) String() string {
	return string(replicaFullId.ShardId) + ":" + strconv.Itoa(int(replicaFullId.ReplicaIdx))
}

type ShardingKey uint32
