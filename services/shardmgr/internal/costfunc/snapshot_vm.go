package costfunc

import (
	"encoding/json"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

/*********************** SnapshotVm **********************/

type SnapshotVm struct {
	SnapshotId     SnapshotId      `json:"snapshot_id"`
	AllShards      []*ShardSnapVm  `json:"all_shards"`
	AllWorkers     []*WorkerVm     `json:"all_workers"`
	AllAssignments []*AssignmentVm `json:"all_assignments"`
}

func NewSnapshotVm(snapshotId SnapshotId) *SnapshotVm {
	return &SnapshotVm{
		SnapshotId: snapshotId,
	}
}

func ParseSnapshotVmFromJson(jsonStr string) *SnapshotVm {
	var snapshotVm SnapshotVm
	err := json.Unmarshal([]byte(jsonStr), &snapshotVm)
	if err != nil {
		ke := kerror.Wrap(err, "ParseSnapshotVmFromJson", "failed to parse snapshot VM from JSON", false)
		panic(ke)
	}
	return &snapshotVm
}

/*********************** ShardSnapVm **********************/

type ShardSnapVm struct {
	ShardId            string       `json:"shard_id"`
	TargetReplicaCount int          `json:"target_count"`
	Replicas           []*ReplicaVm `json:"replicas"`
}

func NewShardSnapVm(shardId string, targetReplicaCount int) *ShardSnapVm {
	return &ShardSnapVm{
		ShardId:            shardId,
		TargetReplicaCount: targetReplicaCount,
	}
}

func ParseShardSnapVmFromJson(jsonStr string) *ShardSnapVm {
	var shardSnapVm ShardSnapVm
	err := json.Unmarshal([]byte(jsonStr), &shardSnapVm)
	if err != nil {
		ke := kerror.Wrap(err, "ParseShardSnapVmFromJson", "failed to parse shard snapshot VM from JSON", false)
		panic(ke)
	}
	return &shardSnapVm
}

/*********************** ReplicaVm **********************/

type ReplicaVm struct {
	ReplicaIdx  data.ReplicaIdx     `json:"idx"`
	Assignments []data.AssignmentId `json:"assigns"`
	LameDuck    bool                `json:"lame_duck,omitempty"`
}

func NewReplicaVm(replicaIdx data.ReplicaIdx) *ReplicaVm {
	return &ReplicaVm{
		ReplicaIdx: replicaIdx,
		LameDuck:   false,
	}
}

func ParseReplicaVmFromJson(jsonStr string) *ReplicaVm {
	var replicaVm ReplicaVm
	err := json.Unmarshal([]byte(jsonStr), &replicaVm)
	if err != nil {
		ke := kerror.Wrap(err, "ParseReplicaVmFromJson", "failed to parse replica VM from JSON", false)
		panic(ke)
	}
	return &replicaVm
}

/*********************** WorkerVm **********************/

type WorkerVm struct {
	WorkerId    data.WorkerId  `json:"worker_id"`
	SessionId   data.SessionId `json:"session_id"`
	Assignments []*AssObj      `json:"assigns"`
	Draining    bool           `json:"draining,omitempty"`
	Offline     bool           `json:"offline,omitempty"`
}

func NewWorkerVm(workerId data.WorkerId, sessionId data.SessionId) *WorkerVm {
	return &WorkerVm{
		WorkerId:  workerId,
		SessionId: sessionId,
	}
}

func ParseWorkerVmFromJson(jsonStr string) *WorkerVm {
	var workerVm WorkerVm
	err := json.Unmarshal([]byte(jsonStr), &workerVm)
	if err != nil {
		ke := kerror.Wrap(err, "ParseWorkerVmFromJson", "failed to parse worker VM from JSON", false)
		panic(ke)
	}
	return &workerVm
}

type AssObj struct {
	ShardId      data.ShardId      `json:"sid"`
	AssignmentId data.AssignmentId `json:"aid"`
}

func NewAssObj(shardId data.ShardId, assignmentId data.AssignmentId) *AssObj {
	return &AssObj{
		ShardId:      shardId,
		AssignmentId: assignmentId,
	}
}

func ParseAssObjFromJson(jsonStr string) *AssObj {
	var assObj AssObj
	err := json.Unmarshal([]byte(jsonStr), &assObj)
	if err != nil {
		ke := kerror.Wrap(err, "ParseAssObjFromJson", "failed to parse assignment object from JSON", false)
		panic(ke)
	}
	return &assObj
}

/*********************** AssignmentVm **********************/

type AssignmentVm struct {
	AssignmentId data.AssignmentId `json:"aid"`
	ShardId      data.ShardId      `json:"sid"`
	ReplicaIdx   data.ReplicaIdx   `json:"idx,omitempty"`
	WorkerId     data.WorkerId     `json:"wid"`
	SessionId    data.SessionId    `json:"ses_id"`
}

func NewAssignmentVm(assignmentId data.AssignmentId, shardId data.ShardId, replicaIdx data.ReplicaIdx, workerId data.WorkerId, sessionId data.SessionId) *AssignmentVm {
	return &AssignmentVm{
		AssignmentId: assignmentId,
		ShardId:      shardId,
		ReplicaIdx:   replicaIdx,
		WorkerId:     workerId,
		SessionId:    sessionId,
	}
}

func ParseAssignmentVmFromJson(jsonStr string) *AssignmentVm {
	var assignmentVm AssignmentVm
	err := json.Unmarshal([]byte(jsonStr), &assignmentVm)
	if err != nil {
		ke := kerror.Wrap(err, "ParseAssignmentVmFromJson", "failed to parse assignment VM from JSON", false)
		panic(ke)
	}
	return &assignmentVm
}
