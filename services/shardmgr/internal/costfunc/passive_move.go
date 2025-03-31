package costfunc

import (
	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

type ApplyMode string

const (
	AM_Strict  ApplyMode = "strict"
	AM_Relaxed ApplyMode = "relaxed"
)

// PassiveMove: something that has already happened. We need to update the system state to reflect that.
// This is different from the active move, which is something we can choose to do or not. For instance, we can choose to assign a shard to a worker or not.
type PassiveMove interface {
	Apply(snapshot *Snapshot, mode ApplyMode) *Snapshot
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
