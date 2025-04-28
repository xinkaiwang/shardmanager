package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

type ReplicaState struct {
	ShardId    data.ShardId
	ReplicaIdx data.ReplicaIdx
	LameDuck   bool // soft delete, we will delete this replica (in housekeeping) when assignments are gone

	Assignments map[data.AssignmentId]common.Unit
}

func NewReplicaState(shardId data.ShardId, replicaIdx data.ReplicaIdx) *ReplicaState {
	return &ReplicaState{
		ShardId:     shardId,
		ReplicaIdx:  replicaIdx,
		Assignments: make(map[data.AssignmentId]common.Unit),
	}
}

func NewReplicaStateByJson(parent *ShardState, replicaStateJson *smgjson.ReplicaStateJson) *ReplicaState {
	replicaState := &ReplicaState{
		ShardId:     parent.ShardId,
		ReplicaIdx:  data.ReplicaIdx(replicaStateJson.ReplicaIdx),
		LameDuck:    common.BoolFromInt8(replicaStateJson.LameDuck),
		Assignments: make(map[data.AssignmentId]common.Unit),
	}

	// for _, assignId := range replicaStateJson.Assignments {
	// 	replicaState.Assignments[data.AssignmentId(assignId)] = common.Unit{}
	// }
	return replicaState
}

func (rs *ReplicaState) ToJson() *smgjson.ReplicaStateJson {
	obj := smgjson.NewReplicaStateJson()
	obj.LameDuck = common.Int8FromBool(rs.LameDuck)
	if rs.ReplicaIdx != 0 {
		idx := int32(rs.ReplicaIdx)
		obj.ReplicaIdx = idx
	}
	// for assignId := range rs.Assignments {
	// 	obj.Assignments = append(obj.Assignments, string(assignId))
	// }
	return obj
}

func (rs *ReplicaState) MarkAsSoftDelete(ctx context.Context) {
	rs.LameDuck = true
}

func (rs *ReplicaState) ToSnapshot(ss *ServiceState) *costfunc.ReplicaSnap {
	obj := costfunc.NewReplicaSnap(rs.ShardId, rs.ReplicaIdx)
	obj.LameDuck = rs.LameDuck
	for assignId := range rs.Assignments {
		assignmentState, ok := ss.AllAssignments[assignId]
		if !ok {
			ke := kerror.Create("AssignmentNotFound", "assignment not found").With("assignmentId", assignId).With("shardId", assignmentState.ShardId).With("replicaIdx", assignmentState.ReplicaIdx).With("workerFullId", assignmentState.WorkerFullId)
			panic(ke)
		}
		if !shouldAssignIncludeInSnapshot(assignmentState, costfunc.ST_Current) {
			continue
		}
		obj.Assignments[assignId] = common.Unit{}
	}
	return obj
}
