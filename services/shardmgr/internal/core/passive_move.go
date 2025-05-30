package core

import (
	"context"
	"strconv"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

// PassiveMove represents an operation that can be applied to ss
type PassiveMove interface {
	costfunc.PassiveMove
	ApplyToSs(ctx context.Context, ss *ServiceState)
}

// RemoveAssignment implements Modifier
type RemoveAssignment struct {
	AssignmentId data.AssignmentId
	ShardId      data.ShardId
	ReplicaIdx   data.ReplicaIdx
	WorkerId     data.WorkerFullId
}

func NewPasMoveRemoveAssignment(assignmentId data.AssignmentId, shardId data.ShardId, replicaIdx data.ReplicaIdx, workerId data.WorkerFullId) *RemoveAssignment {
	return &RemoveAssignment{
		AssignmentId: assignmentId,
		ShardId:      shardId,
		ReplicaIdx:   replicaIdx,
		WorkerId:     workerId,
	}
}
func (move *RemoveAssignment) Apply(snapshot *costfunc.Snapshot) {
	if snapshot.Frozen {
		ke := kerror.Create("RemoveAssignmentApplyFailed", "snapshot is frozen")
		panic(ke)
	}
	// remove assignment from shard/replica
	shardSnap, ok := snapshot.AllShards.Get(move.ShardId)
	if !ok {
		ke := kerror.Create("RemoveAssignmentApplyFailed", "shard not found in snapshot").With("shardId", move.ShardId).With("move", move.Signature())
		panic(ke)
	}
	shardSnap = shardSnap.Clone()
	snapshot.AllShards.Set(move.ShardId, shardSnap)
	replicaSnap, ok := shardSnap.Replicas[move.ReplicaIdx]
	if !ok {
		ke := kerror.Create("RemoveAssignmentApplyFailed", "replica not found in snapshot").With("shardId", move.ShardId).With("replicaIdx", move.ReplicaIdx).With("move", move.Signature())
		panic(ke)
	}
	replicaSnap = replicaSnap.Clone()
	shardSnap.Replicas[move.ReplicaIdx] = replicaSnap
	replicaSnap.LameDuck = true
	delete(replicaSnap.Assignments, move.AssignmentId)
	// remove assignment from AllAssignments
	_, ok = snapshot.AllAssignments.Get(move.AssignmentId)
	if ok {
		// relax mode: if assignment not found, it's ok. For example, when the assignment is already in the process of being removed. in this case, it does not exist in future, but the passive move (triggered by apply eph update) is still trying to remove it.
		snapshot.AllAssignments.Delete(move.AssignmentId)
	}
	// remove assignment from worker
	workerSnap, ok := snapshot.AllWorkers.Get(move.WorkerId)
	if ok {
		workerSnap = workerSnap.Clone()
		snapshot.AllWorkers.Set(move.WorkerId, workerSnap)
		delete(workerSnap.Assignments, move.ShardId)
	}
}

func (move *RemoveAssignment) ApplyToSs(ctx context.Context, ss *ServiceState) {
	// remove assignment from shard/replica
	func() {
		shard, ok := ss.AllShards[move.ShardId]
		if !ok {
			return
		}
		replica, ok := shard.Replicas[move.ReplicaIdx]
		if !ok {
			return
		}
		delete(replica.Assignments, move.AssignmentId)
		replica.LameDuck = true
	}()
	// remove assignment from AllAssignments
	func() {
		_, ok := ss.AllAssignments[move.AssignmentId]
		if !ok {
			return
		}
		delete(ss.AllAssignments, move.AssignmentId)
	}()
	// remove assignment from worker
	func() {
		worker, ok := ss.AllWorkers[move.WorkerId]
		if !ok {
			return
		}
		worker.RemoveAssignment(ctx, move.ShardId)
		// delete(worker.Assignments, move.AssignmentId)
	}()
}

func (move *RemoveAssignment) Signature() string {
	return "RemoveAssignment: " + string(move.AssignmentId) + ":" + string(move.ShardId) + ":" + strconv.Itoa(int(move.ReplicaIdx)) + ":" + move.WorkerId.String()
}

func (move *RemoveAssignment) String() string {
	return move.Signature()
}
