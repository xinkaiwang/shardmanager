package data

import "strconv"

type ShardId string

type ReplicaIdx int

type AssignmentId string

type ReplicaFullId struct {
	ShardId    ShardId
	ReplicaIdx ReplicaIdx
}

func (replicaFullId ReplicaFullId) String() string {
	return string(replicaFullId.ShardId) + ":" + strconv.Itoa(int(replicaFullId.ReplicaIdx))
}
