package costfunc

import (
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

type SnapshotId string

type Snapshot struct {
	SnapshotId  SnapshotId
	CostfuncCfg config.CostfuncConfig
	AllShards   map[data.ShardId]*ShardSnap
	AllWorkers  map[data.WorkerFullId]*WorkerSnap
	AllAssigns  map[data.AssignmentId]*AssignmentSnap
}

type ShardSnap struct {
	ShardId data.ShardId

	Replicas map[data.ReplicaIdx]*ReplicaSnap
}

type ReplicaSnap struct {
	ShardId     data.ShardId
	ReplicaIdx  data.ReplicaIdx
	Assignments map[data.AssignmentId]common.Unit
}

type AssignmentSnap struct {
	ShardId      data.ShardId
	ReplicaIdx   data.ReplicaIdx
	AssignmentId data.AssignmentId
	// WorkerFullId data.WorkerFullId
}

type WorkerSnap struct {
	WorkerFullId data.WorkerFullId
	Assignments  map[data.ShardId]data.AssignmentId
}

func (rep *ReplicaSnap) GetReplicaFullId() data.ReplicaFullId {
	return data.ReplicaFullId{ShardId: rep.ShardId, ReplicaIdx: rep.ReplicaIdx}
}

func (asgn *AssignmentSnap) GetReplicaFullId() data.ReplicaFullId {
	return data.ReplicaFullId{ShardId: asgn.ShardId, ReplicaIdx: asgn.ReplicaIdx}
}

func (worker *WorkerSnap) CanAcceptAssignment(shardId data.ShardId) bool {
	// in case this worker already has this shard (maybe from antoher replica)
	_, ok := worker.Assignments[shardId]
	return !ok
}

func (snap *Snapshot) Clone() *Snapshot {
	clone := &Snapshot{
		SnapshotId:  snap.SnapshotId,
		CostfuncCfg: snap.CostfuncCfg,
		AllShards:   map[data.ShardId]*ShardSnap{},
		AllWorkers:  map[data.WorkerFullId]*WorkerSnap{},
		AllAssigns:  map[data.AssignmentId]*AssignmentSnap{},
	}

	for shardId, shard := range snap.AllShards {
		cloneShard := &ShardSnap{
			ShardId:  shard.ShardId,
			Replicas: map[data.ReplicaIdx]*ReplicaSnap{},
		}
		clone.AllShards[shardId] = cloneShard

		for replicaIdx, replica := range shard.Replicas {
			cloneReplica := &ReplicaSnap{
				ShardId:     replica.ShardId,
				ReplicaIdx:  replica.ReplicaIdx,
				Assignments: map[data.AssignmentId]common.Unit{},
			}
			cloneShard.Replicas[replicaIdx] = cloneReplica

			for asgnId := range replica.Assignments {
				cloneReplica.Assignments[asgnId] = common.Unit{}
			}
		}
	}
	return clone
}
