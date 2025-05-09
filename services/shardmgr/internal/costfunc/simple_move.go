package costfunc

import (
	"context"
	"strings"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

type Move interface {
	GetSignature() string
	Apply(snapshot *Snapshot, mode ApplyMode)
	GetActions(cfg config.ShardConfig) []*Action
}

func (proposal *Proposal) GetSignature() string {
	return proposal.Move.GetSignature()
}

func (proposal *Proposal) GetSize() int {
	return proposal.ProposalSize
}

func (proposal *Proposal) GetEfficiency() Gain {
	return Gain{
		HardScore: proposal.Gain.HardScore,
		SoftScore: proposal.Gain.SoftScore / float64(proposal.ProposalSize),
	}
}

// SimpleMove implements Move
type SimpleMove struct {
	Replica          data.ReplicaFullId
	SrcAssignmentId  data.AssignmentId
	DestAssignmentId data.AssignmentId
	Src              data.WorkerFullId
	Dst              data.WorkerFullId
}

func NewSimpleMove(replica data.ReplicaFullId, srcAssignmentId data.AssignmentId, destAssignmentId data.AssignmentId, src data.WorkerFullId, dst data.WorkerFullId) *SimpleMove {
	return &SimpleMove{
		Replica:          replica,
		SrcAssignmentId:  srcAssignmentId,
		DestAssignmentId: destAssignmentId,
		Src:              src,
		Dst:              dst,
	}
}

func (move *SimpleMove) GetSignature() string {
	return move.Src.String() + "/" + move.Replica.String() + "/" + move.Dst.String()
}

func (move *SimpleMove) Apply(snapshot *Snapshot, mode ApplyMode) {
	snapshot.Unassign(move.Src, move.Replica.ShardId, move.Replica.ReplicaIdx, move.SrcAssignmentId, mode, false)
	snapshot.Assign(move.Replica.ShardId, move.Replica.ReplicaIdx, move.DestAssignmentId, move.Dst, mode)
}

func (move *SimpleMove) GetActions(cfg config.ShardConfig) []*Action {
	var list []*Action
	if cfg.MovePolicy == smgjson.MP_KillBeforeStart {
		// step 1: remove src from routing
		list = append(list, &Action{
			ActionType:           smgjson.AT_RemoveFromRoutingAndSleep,
			ShardId:              move.Replica.ShardId,
			ReplicaIdx:           move.Replica.ReplicaIdx,
			AssignmentId:         move.SrcAssignmentId,
			From:                 move.Src,
			RemoveSrcFromRouting: true,
			SleepMs:              1000, // sleep 1s
		})
		// step 2: drop
		list = append(list, &Action{
			ActionType:    smgjson.AT_DropShard,
			ShardId:       move.Replica.ShardId,
			ReplicaIdx:    move.Replica.ReplicaIdx,
			AssignmentId:  move.SrcAssignmentId,
			From:          move.Src,
			DeleteReplica: false,
		})
		// step 3: add shard
		list = append(list, &Action{
			ActionType:   smgjson.AT_AddShard,
			ShardId:      move.Replica.ShardId,
			ReplicaIdx:   move.Replica.ReplicaIdx,
			AssignmentId: move.DestAssignmentId,
			To:           move.Dst,
		})
		// step 4: add dest to routing
		list = append(list, &Action{
			ActionType:       smgjson.AT_AddToRouting,
			ShardId:          move.Replica.ShardId,
			ReplicaIdx:       move.Replica.ReplicaIdx,
			AssignmentId:     move.DestAssignmentId,
			To:               move.Dst,
			AddDestToRouting: true,
		})
		return list
	} else if cfg.MovePolicy == smgjson.MP_StartBeforeKill {
		// step 1: add shard
		list = append(list, &Action{
			ActionType:   smgjson.AT_AddShard,
			ShardId:      move.Replica.ShardId,
			ReplicaIdx:   move.Replica.ReplicaIdx,
			AssignmentId: move.DestAssignmentId,
			To:           move.Dst,
		})
		// step 2: add dest to routing
		list = append(list, &Action{
			ActionType:       smgjson.AT_AddToRouting,
			ShardId:          move.Replica.ShardId,
			ReplicaIdx:       move.Replica.ReplicaIdx,
			AssignmentId:     move.DestAssignmentId,
			To:               move.Dst,
			AddDestToRouting: true,
		})
		// step 3: remove src from routing
		list = append(list, &Action{
			ActionType:           smgjson.AT_RemoveFromRoutingAndSleep,
			ShardId:              move.Replica.ShardId,
			ReplicaIdx:           move.Replica.ReplicaIdx,
			AssignmentId:         move.SrcAssignmentId,
			From:                 move.Src,
			RemoveSrcFromRouting: true,
			SleepMs:              1000, // sleep 1s
		})
		// step 4: drop
		list = append(list, &Action{
			ActionType:    smgjson.AT_DropShard,
			ShardId:       move.Replica.ShardId,
			ReplicaIdx:    move.Replica.ReplicaIdx,
			AssignmentId:  move.SrcAssignmentId,
			From:          move.Src,
			DeleteReplica: false,
		})
		return list
	} else {
		klogging.Fatal(context.Background()).With("policy", cfg.MovePolicy).Log("UnknownMovePolicy", "")
		return list
	}
}

// AssignMove implements Move
type AssignMove struct {
	Replica      data.ReplicaFullId
	AssignmentId data.AssignmentId
	Worker       data.WorkerFullId
}

func NewAssignMove(replica data.ReplicaFullId, assignmentId data.AssignmentId, worker data.WorkerFullId) *AssignMove {
	return &AssignMove{
		Replica:      replica,
		AssignmentId: assignmentId,
		Worker:       worker,
	}
}

func (move *AssignMove) GetSignature() string {
	return "/" + move.Replica.String() + "/" + move.Worker.String()
}

func (move *AssignMove) Apply(snapshot *Snapshot, mode ApplyMode) {
	snapshot.Assign(move.Replica.ShardId, move.Replica.ReplicaIdx, move.AssignmentId, move.Worker, mode)
}

func (move *AssignMove) GetActions(cfg config.ShardConfig) []*Action {
	var list []*Action
	// step 1: add shard
	list = append(list, &Action{
		ActionType:   smgjson.AT_AddShard,
		ShardId:      move.Replica.ShardId,
		ReplicaIdx:   move.Replica.ReplicaIdx,
		AssignmentId: move.AssignmentId,
		To:           move.Worker,
	})
	// step 2: add dest to routing
	list = append(list, &Action{
		ActionType:       smgjson.AT_AddToRouting,
		ShardId:          move.Replica.ShardId,
		ReplicaIdx:       move.Replica.ReplicaIdx,
		AssignmentId:     move.AssignmentId,
		To:               move.Worker,
		AddDestToRouting: true,
	})
	return list
}

// UnassignMove implements Move
type UnassignMove struct {
	Worker       data.WorkerFullId
	Replica      data.ReplicaFullId
	AssignmentId data.AssignmentId
}

func NewUnassignMove(worker data.WorkerFullId, replica data.ReplicaFullId, assignmentId data.AssignmentId) *UnassignMove {
	return &UnassignMove{
		Worker:       worker,
		Replica:      replica,
		AssignmentId: assignmentId,
	}
}

func (move *UnassignMove) GetSignature() string {
	return move.Worker.String() + "/" + move.Replica.String() + "/"
}

func (move *UnassignMove) Apply(snapshot *Snapshot, mode ApplyMode) {
	snapshot.Unassign(move.Worker, move.Replica.ShardId, move.Replica.ReplicaIdx, move.AssignmentId, mode, true)
}

func (move *UnassignMove) GetActions(cfg config.ShardConfig) []*Action {
	var list []*Action
	// step 1: remove from routing
	list = append(list, &Action{
		ActionType:           smgjson.AT_RemoveFromRoutingAndSleep,
		ShardId:              move.Replica.ShardId,
		ReplicaIdx:           move.Replica.ReplicaIdx,
		AssignmentId:         move.AssignmentId,
		From:                 move.Worker,
		RemoveSrcFromRouting: true,
		SleepMs:              1000, // sleep 1s
	})
	// step 2: drop
	list = append(list, &Action{
		ActionType:    smgjson.AT_DropShard,
		ShardId:       move.Replica.ShardId,
		ReplicaIdx:    move.Replica.ReplicaIdx,
		AssignmentId:  move.AssignmentId,
		From:          move.Worker,
		DeleteReplica: true,
	})
	return list
}

// SwapMove implements Move
type SwapMove struct {
	Replica1 data.ReplicaFullId
	Replica2 data.ReplicaFullId
	Src      data.WorkerFullId
	Dst      data.WorkerFullId
}

func (move *SwapMove) GetSignature() string {
	return move.Replica1.String() + "/" + move.Replica2.String() + "/" + move.Src.String() + "/" + move.Dst.String()
}

// ReplaceMove implements Move
type ReplaceMove struct {
	ReplicaOut data.ReplicaFullId
	ReplicaIn  data.ReplicaFullId
	Worker     data.WorkerFullId
}

func (move *ReplaceMove) GetSignature() string {
	return move.ReplicaOut.String() + "/" + move.ReplicaIn.String() + "/" + move.Worker.String()
}

// worker-2:session-2/shard_2:0/worker-1:session-1
func MoveParseFromSignature(signature string, snapshot *Snapshot) Move {
	parts := strings.Split(signature, "/")
	if len(parts) != 3 {
		ke := kerror.Create("InvalidMoveSignature", "Invalid move").With("signature", signature)
		panic(ke)
	}
	if len(parts[0]) == 0 {
		// AssignMove
		replicaFullId := data.ReplicaFullIdParseFromString(parts[1])
		destWorkerFullId := data.WorkerFullIdParseFromString(parts[2])
		destAssignId := data.AssignmentId("destAssignId")
		return NewAssignMove(replicaFullId, destAssignId, destWorkerFullId)
	} else if len(parts[2]) == 0 {
		// UnassignMove
		srcWorkerFullId := data.WorkerFullIdParseFromString(parts[0])
		replicaFullId := data.ReplicaFullIdParseFromString(parts[1])
		srcAssignId := snapshot.AllWorkers.GetOrPanic(srcWorkerFullId).Assignments[replicaFullId.ShardId]
		return NewUnassignMove(srcWorkerFullId, replicaFullId, srcAssignId)
	} else {
		// SimpleMove
		srcWorkerFullId := data.WorkerFullIdParseFromString(parts[0])
		replicaFullId := data.ReplicaFullIdParseFromString(parts[1])
		destWorkerFullId := data.WorkerFullIdParseFromString(parts[2])
		srcAssignId := snapshot.AllWorkers.GetOrPanic(srcWorkerFullId).Assignments[replicaFullId.ShardId]
		destAssignId := data.AssignmentId("destAssignId")
		return NewSimpleMove(replicaFullId, srcAssignId, destAssignId, srcWorkerFullId, destWorkerFullId)
	}
}
