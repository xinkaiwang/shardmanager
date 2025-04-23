package costfunc

import (
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
	Apply(snapshot *Snapshot, mode ApplyMode) *Snapshot
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

func (move *WorkerDelete) Apply(snapshot *Snapshot, mode ApplyMode) *Snapshot {
	if snapshot.Frozen {
		ke := kerror.Create("WorkerDeleteApplyFailed", "snapshot is frozen")
		panic(ke)
	}
	workerSnap, ok := snapshot.AllWorkers.Get(move.WorkerId)
	if !ok {
		if mode == AM_Strict {
			ke := kerror.Create("WorkerGoneApplyFailed", "worker not found in snapshot")
			panic(ke)
		}
		return snapshot
	}
	// delete all assignments
	for _, assignment := range workerSnap.Assignments {
		snapshot.AllAssignments.Delete(assignment)
	}
	// delete worker
	snapshot.AllWorkers.Delete(move.WorkerId)
	return snapshot
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

func (move *WorkerAdded) Apply(snapshot *Snapshot, mode ApplyMode) *Snapshot {
	_, ok := snapshot.AllWorkers.Get(move.WorkerId)
	if ok {
		if mode == AM_Strict {
			ke := kerror.Create("WorkerAlreadyExists", "worker already exists in snapshot")
			panic(ke)
		}
	}
	snapshot.AllWorkers.Set(move.WorkerId, move.WorkerSnap.Clone()) // make a copy
	return snapshot
}

func (move *WorkerAdded) Signature() string {
	return "WorkerAdded: " + move.WorkerId.String()
}

// WorkerStateChange implements PassiveMove
type WorkerStateChange struct {
	WorkerId data.WorkerFullId
	NewState data.WorkerStateEnum
	HasHat   bool
}

func NewWorkerStateChange(workerId data.WorkerFullId, newState data.WorkerStateEnum, hat bool) *WorkerStateChange {
	return &WorkerStateChange{
		WorkerId: workerId,
		NewState: newState,
		HasHat:   hat,
	}
}

func (move *WorkerStateChange) Apply(snapshot *Snapshot, mode ApplyMode) *Snapshot {
	workerSnap, ok := snapshot.AllWorkers.Get(move.WorkerId)
	if !ok {
		if mode == AM_Strict {
			ke := kerror.Create("WorkerStateChangeFailed", "worker not found in snapshot")
			panic(ke)
		}
		return snapshot
	}
	workerSnap.Draining = move.HasHat

	return snapshot
}

func (move *WorkerStateChange) Signature() string {
	return "WorkerStateChange: " + move.WorkerId.String() + " -> " + string(move.NewState)
}
