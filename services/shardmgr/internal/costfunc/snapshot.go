package costfunc

import (
	"context"
	"fmt"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
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

func NewReplicaSnap(shardId data.ShardId, replicaIdx data.ReplicaIdx) *ReplicaSnap {
	return &ReplicaSnap{
		ShardId:     shardId,
		ReplicaIdx:  replicaIdx,
		Assignments: make(map[data.AssignmentId]common.Unit),
	}
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

func NewWorkerSnap(workerFullId data.WorkerFullId) *WorkerSnap {
	return &WorkerSnap{
		WorkerFullId: workerFullId,
		Assignments:  make(map[data.ShardId]data.AssignmentId),
	}
}

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
	Frozen         bool // 标记当前实例是否已冻结，冻结后不允许修改
	SnapshotId     SnapshotId
	Costfunc       CostFuncProvider
	AllShards      *FastMap[data.ShardId, ShardSnap]
	AllWorkers     *FastMap[data.WorkerFullId, WorkerSnap]
	AllAssignments *FastMap[data.AssignmentId, AssignmentSnap]
	cost           *Cost // nil means not calculated yet
}

func NewSnapshot(ctx context.Context, costfuncCfg config.CostfuncConfig) *Snapshot {
	return &Snapshot{
		Frozen:         false,
		SnapshotId:     SnapshotId(kcommon.RandomString(ctx, 8)),
		Costfunc:       NewCostFuncSimpleProvider(costfuncCfg),
		AllShards:      NewFastMap[data.ShardId, ShardSnap](),
		AllWorkers:     NewFastMap[data.WorkerFullId, WorkerSnap](),
		AllAssignments: NewFastMap[data.AssignmentId, AssignmentSnap](),
	}
}

// Clone: the snapshot should be frozen before using it
func (snap *Snapshot) Clone() *Snapshot {
	if !snap.Frozen {
		ke := kerror.Create("SnapshotNotFrozen", "snapshot not frozen").With("snapshotId", snap.SnapshotId)
		panic(ke)
	}
	clone := &Snapshot{
		SnapshotId:     snap.SnapshotId,
		Costfunc:       snap.Costfunc,
		AllShards:      snap.AllShards.Clone(),
		AllWorkers:     snap.AllWorkers.Clone(),
		AllAssignments: snap.AllAssignments.Clone(),
	}
	return clone
}

func (snap *Snapshot) Freeze() *Snapshot {
	if snap.Frozen {
		ke := kerror.Create("SnapshotAlreadyFrozen", "snapshot already frozen").With("snapshotId", snap.SnapshotId)
		panic(ke)
	}
	snap.AllShards.Freeze()
	snap.AllWorkers.Freeze()
	snap.AllAssignments.Freeze()
	snap.Frozen = true
	return snap
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
	snap.AllAssignments.Set(assignmentId, newAssignmentSnap)
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
	snap.AllAssignments.Delete(assignmentId)
}

func (snap *Snapshot) ApplyMove(move Move) *Snapshot {
	if snap.Frozen {
		ke := kerror.Create("SnapshotAlreadyFrozen", "snapshot already frozen").With("snapshotId", snap.SnapshotId)
		panic(ke)
	}
	move.Apply(snap)
	return snap
}

func (snap *Snapshot) GetCost() Cost {
	if !snap.Frozen {
		snap.Freeze()
	}
	if snap.cost == nil {
		cost := snap.Costfunc.CalCost(snap)
		snap.cost = &cost
	}
	return *snap.cost
}

func (snap *Snapshot) ToShortString() string {
	replicaCount := 0
	snap.AllShards.VisitAll(func(shardId data.ShardId, shardSnap *ShardSnap) {
		replicaCount += len(shardSnap.Replicas)
	})
	return fmt.Sprintf("SnapshotId=%s, Cost=%v, shard=%d, worker=%d, replica=%d, assign=%d", snap.SnapshotId, snap.GetCost(), snap.AllShards.Count(), snap.AllWorkers.Count(), replicaCount, snap.AllAssignments.Count())
}
