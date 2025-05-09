package data

import (
	"strconv"
	"strings"
)

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

func ReplicaFullIdParseFromString(s string) ReplicaFullId {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		panic("Invalid ReplicaFullId string: " + s)
	}
	shardId := ShardId(parts[0])
	replicaIdx, err := strconv.Atoi(parts[1])
	if err != nil {
		panic("Invalid ReplicaFullId string: " + s)
	}
	return NewReplicaFullId(shardId, ReplicaIdx(replicaIdx))
}
