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

// WorkerStateAddRemove implements PassiveMove
type WorkerStateAddRemove struct {
	WorkerId   data.WorkerFullId
	WorkerSnap *WorkerSnap // nil to delete
	reason     string
}

func NewWorkerStateAddRemove(workerId data.WorkerFullId, workerSnap *WorkerSnap, reason string) *WorkerStateAddRemove {
	return &WorkerStateAddRemove{
		WorkerId:   workerId,
		WorkerSnap: workerSnap,
		reason:     reason,
	}
}
func (move *WorkerStateAddRemove) Apply(snapshot *Snapshot) {
	if snapshot.Frozen {
		ke := kerror.Create("WorkerStateAddRemoveApplyFailed", "snapshot is frozen")
		panic(ke)
	}
	if move.WorkerSnap != nil {
		snapshot.AllWorkers.Set(move.WorkerId, move.WorkerSnap)
	} else {
		// delete all assignments
		workerSnap, ok := snapshot.AllWorkers.Get(move.WorkerId)
		if !ok {
			return
		}
		for _, assignment := range workerSnap.Assignments {
			assignmentSnap, ok := snapshot.AllAssignments.Get(assignment)
			if !ok {
				continue
			}
			// delete from shard
			shardSnap, ok := snapshot.AllShards.Get(assignmentSnap.ShardId)
			if !ok {
				continue
			}
			for _, replicaSnap := range shardSnap.Replicas {
				delete(replicaSnap.Assignments, assignment)
			}
			snapshot.AllAssignments.Delete(assignment)
		}
		// delete worker
		snapshot.AllWorkers.Delete(move.WorkerId)
	}
}
func (move *WorkerStateAddRemove) Signature() string {
	return move.reason + ":" + move.WorkerId.String()
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

// ShardStateAddRemove implements PassiveMove
type ShardStateAddRemove struct {
	ShardId data.ShardId
	snap    *ShardSnap // nil to delete
	reason  string
}

func NewShardStateAddRemove(shardId data.ShardId, snap *ShardSnap, reason string) *ShardStateAddRemove {
	return &ShardStateAddRemove{
		ShardId: shardId,
		snap:    snap,
		reason:  reason,
	}
}
func (move *ShardStateAddRemove) Apply(snapshot *Snapshot) {
	if snapshot.Frozen {
		ke := kerror.Create("ShardStateAddRemoveApplyFailed", "snapshot is frozen")
		panic(ke)
	}
	if move.snap != nil {
		snapshot.AllShards.Set(move.ShardId, move.snap)
	} else {
		snapshot.AllShards.Delete(move.ShardId)
	}
}
func (move *ShardStateAddRemove) Signature() string {
	return move.reason + ":" + string(move.ShardId) + ":" + move.snap.String()
}

// ShardStateUpdate implements PassiveMove
type ShardStateUpdate struct {
	ShardId data.ShardId
	fn      func(*ShardSnap)
	reason  string
}

func NewShardStateUpdate(shardId data.ShardId, fn func(*ShardSnap), reason string) *ShardStateUpdate {
	return &ShardStateUpdate{
		ShardId: shardId,
		fn:      fn,
		reason:  reason,
	}
}
func (move *ShardStateUpdate) Apply(snapshot *Snapshot, mode ApplyMode) {
	if snapshot.Frozen {
		ke := kerror.Create("ShardStateUpdateApplyFailed", "snapshot is frozen")
		panic(ke)
	}
	shardSnap, ok := snapshot.AllShards.Get(move.ShardId)
	if !ok {
		ke := kerror.Create("ShardStateUpdateApplyFailed", "shard not found in snapshot").With("shardId", move.ShardId).With("move", move.Signature())
		panic(ke)
	}
	newSnap := shardSnap.Clone()
	move.fn(newSnap)
	snapshot.AllShards.Set(move.ShardId, newSnap)
}
func (move *ShardStateUpdate) Signature() string {
	return move.reason + ":" + string(move.ShardId)
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

// ReplicaStateUpdate implements PassiveMove
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
