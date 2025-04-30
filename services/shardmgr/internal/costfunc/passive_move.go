package costfunc

import (
	"strconv"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

type ApplyMode string

const (
	// strict vs. ralaxed:
	// example: if move is "Worker1.Assignment1 -> nil (unassign)", and the worker1 does not have assignment1, strict mode will panic, relaxed mode will ignore this operation.

	AM_Strict  ApplyMode = "strict"  // strict mode, we will panic if the move is not valid
	AM_Relaxed ApplyMode = "relaxed" // relaxed mode, we will ignore the move if it is not valid
)

// PassiveMove: something that has already happened. We need to update the system state to reflect that. for example, a worker has been deleted or added.
// This is different from the active move, which is something we can choose to do or not. For instance, we can choose to assign a shard to a worker or not.
type PassiveMove interface {
	Apply(snapshot *Snapshot)
	Signature() string // for debugging/logging/metrics purpose only
}

// WorkerDelete implements PassiveMove
type WorkerDelete struct {
	WorkerId data.WorkerFullId
}

func NewWorkerDelete(workerId data.WorkerFullId) *WorkerDelete {
	return &WorkerDelete{
		WorkerId: workerId,
	}
}

func (move *WorkerDelete) Apply(snapshot *Snapshot) {
	if snapshot.Frozen {
		ke := kerror.Create("WorkerDeleteApplyFailed", "snapshot is frozen")
		panic(ke)
	}
	workerSnap, ok := snapshot.AllWorkers.Get(move.WorkerId)
	if !ok {
		return
	}
	// delete all assignments
	for _, assignment := range workerSnap.Assignments {
		snapshot.AllAssignments.Delete(assignment)
	}
	// delete worker
	snapshot.AllWorkers.Delete(move.WorkerId)
}

func (move *WorkerDelete) Signature() string {
	return "WorkerDelete: " + move.WorkerId.String()
}

// WorkerAdded implements PassiveMove
type WorkerAdded struct {
	WorkerId   data.WorkerFullId
	WorkerSnap *WorkerSnap
}

func NewWorkerAdded(workerId data.WorkerFullId, workerSnap *WorkerSnap) *WorkerAdded {
	return &WorkerAdded{
		WorkerId:   workerId,
		WorkerSnap: workerSnap,
	}
}

func (move *WorkerAdded) Apply(snapshot *Snapshot) *Snapshot {
	_, ok := snapshot.AllWorkers.Get(move.WorkerId)
	if ok {
		// worker already exists, no need to add
		return snapshot
	}
	snapshot.AllWorkers.Set(move.WorkerId, move.WorkerSnap.Clone()) // make a copy
	return snapshot
}

func (move *WorkerAdded) Signature() string {
	return "WorkerAdded: " + move.WorkerId.String()
}

// WorkerStateUpdate implements PassiveMove
type WorkerStateUpdate struct {
	WorkerId data.WorkerFullId
	fn       func(*WorkerSnap)
	reason   string
}

func NewWorkerStateUpdate(workerId data.WorkerFullId, fn func(*WorkerSnap), reason string) *WorkerStateUpdate {
	return &WorkerStateUpdate{
		WorkerId: workerId,
		fn:       fn,
		reason:   reason,
	}
}

func (move *WorkerStateUpdate) Apply(snapshot *Snapshot) {
	if snapshot.Frozen {
		ke := kerror.Create("WorkerStateUpdateApplyFailed", "snapshot is frozen")
		panic(ke)
	}
	workerSnap, ok := snapshot.AllWorkers.Get(move.WorkerId)
	if !ok {
		ke := kerror.Create("WorkerStateUpdateApplyFailed", "worker not found in snapshot").With("workerId", move.WorkerId).With("move", move.Signature())
		panic(ke)
	}
	newSnap := workerSnap.Clone()
	move.fn(newSnap)
	snapshot.AllWorkers.Set(move.WorkerId, newSnap)
}
func (move *WorkerStateUpdate) Signature() string {
	return move.reason + ":" + move.WorkerId.String()
}

// WorkerStateChange implements PassiveMove
type WorkerStateChange struct {
	WorkerId data.WorkerFullId
	NewState data.WorkerStateEnum
	Draining bool
}

func NewWorkerStateChange(workerId data.WorkerFullId, newState data.WorkerStateEnum, draining bool) *WorkerStateChange {
	return &WorkerStateChange{
		WorkerId: workerId,
		NewState: newState,
		Draining: draining,
	}
}

func (move *WorkerStateChange) Apply(snapshot *Snapshot) {
	workerSnap, ok := snapshot.AllWorkers.Get(move.WorkerId)
	if !ok {
		return
	}
	if move.NewState == data.WS_Deleted || move.NewState == data.WS_Offline_dead {
		// delete all assignments (rare case)
		for shardId, assignmentId := range workerSnap.Assignments {
			snapshot.AllAssignments.Delete(assignmentId)
			// delete from shard
			shardSnap, ok := snapshot.AllShards.Get(shardId)
			if !ok {
				continue
			}
			for _, replicaSnap := range shardSnap.Replicas {
				delete(replicaSnap.Assignments, assignmentId)
			}
		}
		// delete worker
		snapshot.AllWorkers.Delete(move.WorkerId)
		return
	}
	workerSnap.Draining = move.Draining
}

func (move *WorkerStateChange) Signature() string {
	return "WorkerStateChange: " + move.WorkerId.String() + " -> " + string(move.NewState)
}

// ShardStateChange implements PassiveMove
type ShardStateChange struct {
	ShardId   data.ShardId
	ShardSnap *ShardSnap // nil to delete
	// NewState  bool // true: add, false: delete
}

func NewShardStateChange(shardId data.ShardId, newSnap *ShardSnap) *ShardStateChange {
	return &ShardStateChange{
		ShardId:   shardId,
		ShardSnap: newSnap,
	}
}

func (move *ShardStateChange) Apply(snapshot *Snapshot) {
	if snapshot.Frozen {
		ke := kerror.Create("ShardStateChangeApplyFailed", "snapshot is frozen")
		panic(ke)
	}
	if move.ShardSnap != nil {
		snapshot.AllShards.Set(move.ShardId, move.ShardSnap)
	} else {
		snapshot.AllShards.Delete(move.ShardId)
	}
}

func (move *ShardStateChange) Signature() string {
	return "ShardStateChange: " + string(move.ShardId) + ":" + move.ShardSnap.String()
}

// ReplicaAddRemove implements PassiveMove
type ReplicaAddRemove struct {
	ShardId    data.ShardId
	ReplicaIdx data.ReplicaIdx
	NewState   bool // true: add, false: delete
}

func NewReplicaAddRemove(shardId data.ShardId, replicaIdx data.ReplicaIdx, newState bool) *ReplicaAddRemove {
	return &ReplicaAddRemove{
		ShardId:    shardId,
		ReplicaIdx: replicaIdx,
		NewState:   newState,
	}
}

func (move *ReplicaAddRemove) Apply(snapshot *Snapshot) {
	if snapshot.Frozen {
		ke := kerror.Create("ReplicaStateChangeApplyFailed", "snapshot is frozen")
		panic(ke)
	}
	shardSnap, ok := snapshot.AllShards.Get(move.ShardId)
	if !ok {
		ke := kerror.Create("ReplicaStateChangeApplyFailed", "shard not found in snapshot").With("shardId", move.ShardId).With("move", move.Signature())
		panic(ke)
	}
	shardSnap = shardSnap.Clone()
	snapshot.AllShards.Set(move.ShardId, shardSnap)
	_, ok = shardSnap.Replicas[move.ReplicaIdx]
	if move.NewState {
		if ok {
			return
		}
		replicaSnap := NewReplicaSnap(move.ShardId, move.ReplicaIdx)
		shardSnap.Replicas[move.ReplicaIdx] = replicaSnap
	} else {
		if !ok {
			return
		}
		delete(shardSnap.Replicas, move.ReplicaIdx)
	}
}

func (move *ReplicaAddRemove) Signature() string {
	return "ReplicaStateChange: " + string(move.ShardId) + ":" + strconv.Itoa(int(move.ReplicaIdx)) + " -> " + strconv.FormatBool(move.NewState)
}

type ReplicaStateUpdate struct {
	ShardId    data.ShardId
	ReplicaIdx data.ReplicaIdx
	fn         func(*ReplicaSnap)
	reason     string
}

func NewReplicaStateUpdate(shardId data.ShardId, replicaIdx data.ReplicaIdx, fn func(*ReplicaSnap), reason string) *ReplicaStateUpdate {
	return &ReplicaStateUpdate{
		ShardId:    shardId,
		ReplicaIdx: replicaIdx,
		fn:         fn,
		reason:     reason,
	}
}

func (move *ReplicaStateUpdate) Apply(snapshot *Snapshot) {
	if snapshot.Frozen {
		ke := kerror.Create("ReplicaStateChangeApplyFailed", "snapshot is frozen")
		panic(ke)
	}
	shardSnap, ok := snapshot.AllShards.Get(move.ShardId)
	if !ok {
		ke := kerror.Create("ReplicaStateChangeApplyFailed", "shard not found in snapshot").With("shardId", move.ShardId).With("move", move.Signature())
		panic(ke)
	}
	replicaSnap, ok := shardSnap.Replicas[move.ReplicaIdx]
	if !ok {
		ke := kerror.Create("ReplicaStateChangeApplyFailed", "replica not found in snapshot").With("shardId", move.ShardId).With("replicaIdx", move.ReplicaIdx).With("move", move.Signature())
		panic(ke)
	}
	newShard := shardSnap.Clone()
	snapshot.AllShards.Set(move.ShardId, newShard)
	newReplicaSnap := replicaSnap.Clone()
	newShard.Replicas[move.ReplicaIdx] = newReplicaSnap
	move.fn(newReplicaSnap)
}

func (move *ReplicaStateUpdate) Signature() string {
	return move.reason + string(move.ShardId) + ":" + strconv.Itoa(int(move.ReplicaIdx))
}
