package costfunc

import "github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"

type SnapshotId string

type Snapshot struct {
	SnapshotId  SnapshotId
	CostfuncCfg CostfuncCfg
	AllShards   map[data.ShardId]*ShardSnap
	AllWorkers  map[data.WorkerFullId]*WorkerSnap
}

type ShardSnap struct {
	ShardId data.ShardId

	Replicas map[data.ReplicaIdx]*ReplicaSnap
}

type ReplicaSnap struct {
	ShardId     data.ShardId
	ReplicaIdx  data.ReplicaIdx
	Assignments []*AssignmentSnap
}

type AssignmentSnap struct {
	ShardId      data.ShardId
	ReplicaIdx   data.ReplicaIdx
	AssignmentId data.AssignmentId
	WorkerFullId data.WorkerFullId
}

type WorkerSnap struct {
	WorkerFullId data.WorkerFullId
	Assignments  map[data.AssignmentId]*AssignmentSnap
}

func (rep *ReplicaSnap) GetReplicaFullId() data.ReplicaFullId {
	return data.ReplicaFullId{ShardId: rep.ShardId, ReplicaIdx: rep.ReplicaIdx}
}

func (asgn *AssignmentSnap) GetReplicaFullId() data.ReplicaFullId {
	return data.ReplicaFullId{ShardId: asgn.ShardId, ReplicaIdx: asgn.ReplicaIdx}
}
