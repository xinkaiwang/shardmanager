package costfunc

import (
	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

/*********************** ShardSnap **********************/

// ShardSnap: implements TypeT2
type ShardSnap struct {
	ShardId  data.ShardId
	Replicas map[data.ReplicaIdx]*ReplicaSnap
}

func (ss ShardSnap) IsValueTypeT2() {}

func NewShardSnap(shardId data.ShardId) *ShardSnap {
	return &ShardSnap{
		ShardId:  shardId,
		Replicas: make(map[data.ReplicaIdx]*ReplicaSnap),
	}
}

func (ss *ShardSnap) Clone() *ShardSnap {
	clone := &ShardSnap{
		ShardId:  ss.ShardId,
		Replicas: make(map[data.ReplicaIdx]*ReplicaSnap),
	}
	for replicaIdx, replicaSnap := range ss.Replicas {
		clone.Replicas[replicaIdx] = replicaSnap
	}
	return clone
}

/*********************** ReplicaSnap **********************/
type ReplicaSnap struct {
	ShardId     data.ShardId
	ReplicaIdx  data.ReplicaIdx
	Assignments map[data.AssignmentId]common.Unit
	LameDuck    bool
}

func (rep *ReplicaSnap) GetReplicaFullId() data.ReplicaFullId {
	return data.ReplicaFullId{ShardId: rep.ShardId, ReplicaIdx: rep.ReplicaIdx}
}

func (rep *ReplicaSnap) Clone() *ReplicaSnap {
	clone := &ReplicaSnap{
		ShardId:     rep.ShardId,
		ReplicaIdx:  rep.ReplicaIdx,
		Assignments: make(map[data.AssignmentId]common.Unit),
	}
	for assignmentId := range rep.Assignments {
		clone.Assignments[assignmentId] = common.Unit{}
	}
	return clone
}

/*********************** AssignmentSnap **********************/

// AssignmentSnap: implements TypeT2
type AssignmentSnap struct {
	ShardId      data.ShardId
	ReplicaIdx   data.ReplicaIdx
	AssignmentId data.AssignmentId
	WorkerFullId data.WorkerFullId // the benefit of have this info: unassign solver can keep list a assignment as candidate
}

func (asgn AssignmentSnap) IsValueTypeT2() {}

func NewAssignmentSnap(shardId data.ShardId, replicaIdx data.ReplicaIdx, assignmentId data.AssignmentId, workerFullId data.WorkerFullId) *AssignmentSnap {
	return &AssignmentSnap{
		ShardId:      shardId,
		ReplicaIdx:   replicaIdx,
		AssignmentId: assignmentId,
		WorkerFullId: workerFullId,
	}
}

func (asgn *AssignmentSnap) GetReplicaFullId() data.ReplicaFullId {
	return data.ReplicaFullId{ShardId: asgn.ShardId, ReplicaIdx: asgn.ReplicaIdx}
}

/*********************** WorkerSnap **********************/

// WorkerSnap: implements TypeT2
type WorkerSnap struct {
	WorkerFullId data.WorkerFullId
	Assignments  map[data.ShardId]data.AssignmentId
}

func (ws WorkerSnap) IsValueTypeT2() {}

func (worker *WorkerSnap) CanAcceptAssignment(shardId data.ShardId) bool {
	// in case this worker already has this shard (maybe from antoher replica)
	_, ok := worker.Assignments[shardId]
	return !ok
}

func (worker *WorkerSnap) Clone() *WorkerSnap {
	clone := &WorkerSnap{
		WorkerFullId: worker.WorkerFullId,
		Assignments:  make(map[data.ShardId]data.AssignmentId),
	}
	for shardId, assignmentId := range worker.Assignments {
		clone.Assignments[shardId] = assignmentId
	}
	return clone
}

/*********************** Snapshot **********************/
type SnapshotId string

type Snapshot struct {
	SnapshotId  SnapshotId
	CostfuncCfg config.CostfuncConfig
	AllShards   *FastMap[data.ShardId, ShardSnap]
	AllWorkers  *FastMap[data.WorkerFullId, WorkerSnap]
	AllAssigns  *FastMap[data.AssignmentId, AssignmentSnap]
	// AllAssigns map[data.AssignmentId]*AssignmentSnap
}

func (snap *Snapshot) Clone() *Snapshot {
	clone := &Snapshot{
		SnapshotId:  snap.SnapshotId,
		CostfuncCfg: snap.CostfuncCfg,
		AllShards:   snap.AllShards.Clone(),
		AllWorkers:  snap.AllWorkers.Clone(),
		AllAssigns:  snap.AllAssigns.Clone(),
	}
	return clone
}

func (snap *Snapshot) Assign(shardId data.ShardId, replicaIdx data.ReplicaIdx, assignmentId data.AssignmentId, workerFullId data.WorkerFullId) {
	shardSnap, ok := snap.AllShards.Get(shardId)
	if !ok {
		ke := kerror.Create("ShardNotFound", "shard not found").With("shardId", shardId).With("replicaIdx", replicaIdx).With("assignmentId", assignmentId)
		panic(ke)
	}
	replicaSnap, ok := shardSnap.Replicas[replicaIdx]
	if !ok {
		ke := kerror.Create("ReplicaNotFound", "replica not found").With("shardId", shardId).With("replicaIdx", replicaIdx).With("assignmentId", assignmentId)
		panic(ke)
	}
	// update shardSnap
	newReplicaSnap := replicaSnap.Clone()
	newReplicaSnap.Assignments[assignmentId] = common.Unit{}
	newShardSnap := shardSnap.Clone()
	newShardSnap.Replicas[replicaIdx] = newReplicaSnap
	snap.AllShards.Set(shardId, newShardSnap)
	// update workerSnap
	workerSnap, ok := snap.AllWorkers.Get(workerFullId)
	if !ok {
		ke := kerror.Create("WorkerNotFound", "worker not found").With("workerFullId", workerFullId).With
		panic(ke)
	}
	newWorkerSnap := workerSnap.Clone()
	newWorkerSnap.Assignments[shardId] = assignmentId
	snap.AllWorkers.Set(workerFullId, newWorkerSnap)
	// update assignmentSnap
	newAssignmentSnap := NewAssignmentSnap(shardId, replicaIdx, assignmentId, workerFullId)
	snap.AllAssigns.Set(assignmentId, newAssignmentSnap)
}

func (snap *Snapshot) Unassign(workerFullId data.WorkerFullId, shardId data.ShardId, replicaIdx data.ReplicaIdx, assignmentId data.AssignmentId) {
	// update workerSnap
	workerSnap, ok := snap.AllWorkers.Get(workerFullId)
	if !ok {
		ke := kerror.Create("WorkerNotFound", "worker not found").With("workerFullId", workerFullId).With
		panic(ke)
	}
	newWorkerSnap := workerSnap.Clone()
	delete(newWorkerSnap.Assignments, shardId)
	snap.AllWorkers.Set(workerFullId, newWorkerSnap)
	// update shardSnap
	shardSnap, ok := snap.AllShards.Get(shardId)
	if !ok {
		ke := kerror.Create("ShardNotFound", "shard not found").With("shardId", shardId).With("replicaIdx", replicaIdx).With("assignmentId", assignmentId)
		panic(ke)
	}
	replicaSnap, ok := shardSnap.Replicas[replicaIdx]
	if !ok {
		ke := kerror.Create("ReplicaNotFound", "replica not found").With("shardId", shardId).With("replicaIdx", replicaIdx).With("assignmentId", assignmentId)
		panic(ke)
	}
	newReplicaSnap := replicaSnap.Clone()
	delete(newReplicaSnap.Assignments, assignmentId)
	newShardSnap := shardSnap.Clone()
	newShardSnap.Replicas[replicaIdx] = newReplicaSnap
	snap.AllShards.Set(shardId, newShardSnap)
	// update assignmentSnap
	snap.AllAssigns.Delete(assignmentId)
}
