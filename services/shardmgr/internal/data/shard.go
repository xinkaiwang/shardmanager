package data

type ShardId string

type ReplicaIdx int

type AssignmentId string

type ReplicaFullId struct {
	ShardId    ShardId
	ReplicaIdx ReplicaIdx
}
