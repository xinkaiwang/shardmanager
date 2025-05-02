package costfunc

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

/*********************** ShardSnap **********************/

// ShardSnap: implements TypeT2
type ShardSnap struct {
	ShardId            data.ShardId
	TargetReplicaCount int
	Replicas           map[data.ReplicaIdx]*ReplicaSnap
}

func (ss ShardSnap) IsValueTypeT2() {}

func NewShardSnap(shardId data.ShardId, initReplicaCount int) *ShardSnap {
	shard := &ShardSnap{
		ShardId:            shardId,
		TargetReplicaCount: initReplicaCount,
		Replicas:           make(map[data.ReplicaIdx]*ReplicaSnap, initReplicaCount),
	}
	for i := 0; i < initReplicaCount; i++ {
		replica := NewReplicaSnap(shardId, data.ReplicaIdx(i))
		shard.Replicas[data.ReplicaIdx(i)] = replica
	}
	return shard
}

func (ss *ShardSnap) Clone() *ShardSnap {
	clone := &ShardSnap{
		ShardId:            ss.ShardId,
		TargetReplicaCount: ss.TargetReplicaCount,
		Replicas:           make(map[data.ReplicaIdx]*ReplicaSnap),
	}
	for replicaIdx, replicaSnap := range ss.Replicas {
		clone.Replicas[replicaIdx] = replicaSnap
	}
	return clone
}

func (ss ShardSnap) CompareWith(other TypeT2) []string {
	if other, ok := other.(ShardSnap); ok {
		return ss.Compare(&other)
	}
	return []string{"ShardSnap:CompareWith:otherIsNotShardSnap"}
}

func (ss *ShardSnap) Compare(other *ShardSnap) []string {
	var diff []string
	if ss.ShardId != other.ShardId {
		diff = append(diff, "ShardId")
	}
	if len(ss.Replicas) != len(other.Replicas) {
		diff = append(diff, "ShardSnap:ReplicaCount")
	}
	for replicaIdx, replicaSnap := range ss.Replicas {
		if otherReplicaSnap, ok := other.Replicas[replicaIdx]; !ok {
			diff = append(diff, "ShardSnap:Replicas:missingFromOther:"+strconv.Itoa(int(replicaIdx)))
		} else {
			diff = append(diff, replicaSnap.Compare(otherReplicaSnap)...)
		}
	}
	return diff
}

func (ss *ShardSnap) ToJson() map[string]interface{} {
	obj := make(map[string]interface{})
	obj["ShardId"] = ss.ShardId
	obj["Replicas"] = make([]map[string]interface{}, 0)
	for replicaIdx, replicaSnap := range ss.Replicas {
		replicaObj := replicaSnap.ToJson()
		replicaObj["ReplicaIdx"] = replicaIdx
		obj["Replicas"] = append(obj["Replicas"].([]map[string]interface{}), replicaObj)
	}
	return obj
}

func (ss *ShardSnap) String() string {
	jsonStr, err := json.Marshal(ss.ToJson())
	if err != nil {
		klogging.Fatal(context.Background()).WithError(err).Log("ShardSnap:ToJson", "error")
	}
	return string(jsonStr)
}

/*********************** ReplicaSnap **********************/
type ReplicaSnap struct {
	ShardId     data.ShardId
	ReplicaIdx  data.ReplicaIdx
	Assignments map[data.AssignmentId]common.Unit

	// Lame duck: set to remove this replica.
	// How it works: A regular (non-LameDuck) replica will got hard score penalty (H1) if it has no assignment.
	// But once set as LameDuck, it will not got those penalty. Thus eventrually it will be unassigned (due to soft score incentives).
	LameDuck bool
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
		LameDuck:    rep.LameDuck,
		Assignments: make(map[data.AssignmentId]common.Unit),
	}
	for assignmentId := range rep.Assignments {
		clone.Assignments[assignmentId] = common.Unit{}
	}
	return clone
}

func (rep *ReplicaSnap) Compare(other *ReplicaSnap) []string {
	var diff []string
	if rep.ShardId != other.ShardId {
		diff = append(diff, "ShardId")
	}
	if rep.ReplicaIdx != other.ReplicaIdx {
		diff = append(diff, "ReplicaIdx")
	}
	if rep.LameDuck != other.LameDuck {
		diff = append(diff, "ReplicaSnap:LameDuck:"+string(rep.ShardId))
	}
	for assignmentId := range rep.Assignments {
		if _, ok := other.Assignments[assignmentId]; !ok {
			diff = append(diff, "ReplicaSnap:Assignments:missingFromOther:"+string(assignmentId))
		}
	}
	for assignmentId := range other.Assignments {
		if _, ok := rep.Assignments[assignmentId]; !ok {
			diff = append(diff, "ReplicaSnap:Assignments:missingFromSelf:"+string(assignmentId))
		}
	}
	return diff
}

func (rep *ReplicaSnap) ToJson() map[string]interface{} {
	obj := make(map[string]interface{})
	obj["ShardId"] = rep.ShardId
	obj["ReplicaIdx"] = rep.ReplicaIdx
	obj["LameDuck"] = common.Int8FromBool(rep.LameDuck)
	obj["Assignments"] = make([]string, 0)
	for assignmentId := range rep.Assignments {
		obj["Assignments"] = append(obj["Assignments"].([]string), string(assignmentId))
	}
	return obj
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

func (asgn *AssignmentSnap) String() string {
	return fmt.Sprintf("AssignmentSnap: %s:%d:%s:%s", asgn.ShardId, asgn.ReplicaIdx, asgn.AssignmentId, asgn.WorkerFullId)
}

func (asgn *AssignmentSnap) GetReplicaFullId() data.ReplicaFullId {
	return data.ReplicaFullId{ShardId: asgn.ShardId, ReplicaIdx: asgn.ReplicaIdx}
}

func (ss AssignmentSnap) CompareWith(other TypeT2) []string {
	if other, ok := other.(AssignmentSnap); ok {
		return ss.Compare(&other)
	}
	return []string{"AssignmentSnap:CompareWith:otherIsNotAssignmentSnap"}
}

func (asgn *AssignmentSnap) Compare(other *AssignmentSnap) []string {
	var diff []string
	if asgn.ShardId != other.ShardId {
		diff = append(diff, "ShardId")
	}
	if asgn.ReplicaIdx != other.ReplicaIdx {
		diff = append(diff, "ReplicaIdx")
	}
	if asgn.AssignmentId != other.AssignmentId {
		diff = append(diff, "AssignmentId")
	}
	if asgn.WorkerFullId != other.WorkerFullId {
		diff = append(diff, "WorkerFullId")
	}
	return diff
}

func (asgn *AssignmentSnap) ToJson() map[string]interface{} {
	obj := make(map[string]interface{})
	obj["ShardId"] = asgn.ShardId
	obj["ReplicaIdx"] = asgn.ReplicaIdx
	obj["AssignmentId"] = asgn.AssignmentId
	obj["WorkerFullId"] = asgn.WorkerFullId
	return obj
}

/*********************** WorkerSnap **********************/

// WorkerSnap: implements TypeT2
type WorkerSnap struct {
	WorkerFullId data.WorkerFullId
	Draining     bool
	Assignments  map[data.ShardId]data.AssignmentId
}

func (ws WorkerSnap) IsValueTypeT2() {}

func NewWorkerSnap(workerFullId data.WorkerFullId) *WorkerSnap {
	return &WorkerSnap{
		WorkerFullId: workerFullId,
		Assignments:  make(map[data.ShardId]data.AssignmentId),
	}
}

func (ss WorkerSnap) CompareWith(other TypeT2) []string {
	if other, ok := other.(WorkerSnap); ok {
		return ss.Compare(&other)
	}
	return []string{"WorkerSnap:CompareWith:otherIsNotWorkerSnap"}
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

func (worker *WorkerSnap) Compare(other *WorkerSnap) []string {
	var diff []string
	if worker.WorkerFullId != other.WorkerFullId {
		return []string{"WorkerFullId"}
	}
	for shardId, assignmentId := range worker.Assignments {
		if otherAssignmentId, ok := other.Assignments[shardId]; !ok && assignmentId != otherAssignmentId {
			diff = append(diff, worker.WorkerFullId.String()+":missingFromOther:"+string(shardId)+":"+string(assignmentId))
		}
	}
	for shardId, assignmentId := range other.Assignments {
		if _, ok := worker.Assignments[shardId]; !ok && assignmentId != worker.Assignments[shardId] {
			diff = append(diff, worker.WorkerFullId.String()+":missingFromSelf:"+string(shardId)+":"+string(assignmentId))
		}
	}
	return diff
}

func (worker *WorkerSnap) ToJson() map[string]interface{} {
	obj := make(map[string]interface{})
	obj["WorkerFullId"] = worker.WorkerFullId
	obj["Draining"] = common.Int8FromBool(worker.Draining)
	obj["Assignments"] = make([]map[string]interface{}, 0)
	for shardId, assignmentId := range worker.Assignments {
		assignObj := make(map[string]interface{})
		assignObj["ShardId"] = shardId
		assignObj["AssignmentId"] = assignmentId
		obj["Assignments"] = append(obj["Assignments"].([]map[string]interface{}), assignObj)
	}
	return obj
}

/*********************** Snapshot **********************/
type SnapshotId string

type SnapshotType string

const (
	ST_Current SnapshotType = "current"
	ST_Future  SnapshotType = "future"
)

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
		SnapshotId:     SnapshotId(kcommon.RandomString(context.Background(), 8)),
		Costfunc:       snap.Costfunc,
		AllShards:      snap.AllShards.Clone(),
		AllWorkers:     snap.AllWorkers.Clone(),
		AllAssignments: snap.AllAssignments.Clone(),
	}
	return clone
}

func (snap *Snapshot) Freeze() {
	if snap.Frozen {
		ke := kerror.Create("SnapshotAlreadyFrozen", "snapshot already frozen").With("snapshotId", snap.SnapshotId)
		panic(ke)
	}
	snap.AllShards.Freeze()
	snap.AllWorkers.Freeze()
	snap.AllAssignments.Freeze()
	snap.Frozen = true
}

func (snap *Snapshot) CompactAndFreeze() *Snapshot {
	snap.AllShards = snap.AllShards.Compact()
	snap.AllWorkers = snap.AllWorkers.Compact()
	snap.AllAssignments = snap.AllAssignments.Compact()
	snap.Freeze()
	return snap
}

func (snap *Snapshot) Assign(shardId data.ShardId, replicaIdx data.ReplicaIdx, assignmentId data.AssignmentId, workerFullId data.WorkerFullId, mode ApplyMode) {
	shardSnap, ok := snap.AllShards.Get(shardId)
	if !ok {
		if mode == AM_Strict {
			ke := kerror.Create("ShardNotFound", "shard not found").With("shardId", shardId).With("replicaIdx", replicaIdx).With("assignmentId", assignmentId)
			panic(ke)
		} else if mode == AM_Relaxed {
			return
		} else {
			klogging.Fatal(context.Background()).With("mode", mode).Log("unknownMoveMode", "mode")
		}
	}
	workerSnap, ok := snap.AllWorkers.Get(workerFullId)
	if !ok {
		if mode == AM_Strict {
			ke := kerror.Create("WorkerNotFound", "worker not found").With("workerFullId", workerFullId).With
			panic(ke)
		} else if mode == AM_Relaxed {
			return
		} else {
			klogging.Fatal(context.Background()).With("mode", mode).Log("unknownMoveMode", "mode")
		}
	}
	if oldAssignId, ok := workerSnap.Assignments[shardId]; ok {
		if mode == AM_Strict {
			ke := kerror.Create("WorkerAlreadyHasShard", "worker already has shard").With("workerFullId", workerFullId).With("shardId", shardId).With("newReplica", replicaIdx).With("oldAssigId", oldAssignId).With("newAssigId", assignmentId)
			panic(ke)
		} else if mode == AM_Relaxed {
			klogging.Warning(context.Background()).With("workerFullId", workerFullId).With("shardId", shardId).With("newReplica", replicaIdx).With("oldAssigId", oldAssignId).With("newAssigId", assignmentId).Log("WorkerAlreadyHasShard", "this should never happen")
			// ignore and continue
		} else {
			klogging.Fatal(context.Background()).With("mode", mode).Log("unknownMoveMode", "mode")
		}
	}
	replicaSnap, ok := shardSnap.Replicas[replicaIdx]
	if !ok {
		// looks like this is a new replica
		replicaSnap = NewReplicaSnap(shardId, replicaIdx)
	}
	// update shardSnap
	newReplicaSnap := replicaSnap.Clone()
	newReplicaSnap.Assignments[assignmentId] = common.Unit{}
	newShardSnap := shardSnap.Clone()
	newShardSnap.Replicas[replicaIdx] = newReplicaSnap
	snap.AllShards.Set(shardId, newShardSnap)
	// update workerSnap
	newWorkerSnap := workerSnap.Clone()
	newWorkerSnap.Assignments[shardId] = assignmentId
	snap.AllWorkers.Set(workerFullId, newWorkerSnap)
	// update assignmentSnap
	newAssignmentSnap := NewAssignmentSnap(shardId, replicaIdx, assignmentId, workerFullId)
	snap.AllAssignments.Set(assignmentId, newAssignmentSnap)
}

func (snap *Snapshot) Unassign(workerFullId data.WorkerFullId, shardId data.ShardId, replicaIdx data.ReplicaIdx, assignmentId data.AssignmentId, mode ApplyMode, removeReplica bool) {
	// update workerSnap
	workerSnap, ok := snap.AllWorkers.Get(workerFullId)
	if !ok {
		if mode == AM_Strict {
			ke := kerror.Create("WorkerNotFound", "worker not found").With("workerFullId", workerFullId).With
			panic(ke)
		} else if mode == AM_Relaxed {
			return
		} else {
			klogging.Fatal(context.Background()).With("mode", mode).Log("unknownMoveMode", "mode")
		}
	}
	shardSnap, ok := snap.AllShards.Get(shardId)
	if !ok {
		if mode == AM_Strict {
			ke := kerror.Create("ShardNotFound", "shard not found").With("shardId", shardId).With("replicaIdx", replicaIdx).With("assignmentId", assignmentId)
			panic(ke)
		} else if mode == AM_Relaxed {
			return
		} else {
			klogging.Fatal(context.Background()).With("mode", mode).Log("unknownMoveMode", "mode")
		}
	}
	replicaSnap, ok := shardSnap.Replicas[replicaIdx]
	if !ok {
		if mode == AM_Strict {
			ke := kerror.Create("ReplicaNotFound", "replica not found").With("shardId", shardId).With("replicaIdx", replicaIdx).With("assignmentId", assignmentId)
			panic(ke)
		} else if mode == AM_Relaxed {
			return
		} else {
			klogging.Fatal(context.Background()).With("mode", mode).Log("unknownMoveMode", "mode")
		}
	}
	_, ok = snap.AllAssignments.Get(assignmentId)
	if !ok {
		if mode == AM_Strict {
			ke := kerror.Create("AssignmentNotFound", "assignment not found").With("assignmentId", assignmentId).With("shardId", shardId).With("replicaIdx", replicaIdx)
			panic(ke)
		} else if mode == AM_Relaxed {
			return
		} else {
			klogging.Fatal(context.Background()).With("mode", mode).Log("unknownMoveMode", "mode")
		}
	}
	newWorkerSnap := workerSnap.Clone()
	delete(newWorkerSnap.Assignments, shardId)
	snap.AllWorkers.Set(workerFullId, newWorkerSnap)
	// update shardSnap
	newReplicaSnap := replicaSnap.Clone()
	if removeReplica {
		newReplicaSnap.LameDuck = true
	}
	delete(newReplicaSnap.Assignments, assignmentId)
	newShardSnap := shardSnap.Clone()
	newShardSnap.Replicas[replicaIdx] = newReplicaSnap
	snap.AllShards.Set(shardId, newShardSnap)
	// update assignmentSnap
	snap.AllAssignments.Delete(assignmentId)
}

func (snap *Snapshot) ApplyMove(move Move, mode ApplyMode) *Snapshot {
	if snap.Frozen {
		ke := kerror.Create("SnapshotAlreadyFrozen", "trying to apply on frozen snapshot").With("snapshotId", snap.SnapshotId)
		panic(ke)
	}
	move.Apply(snap, mode)
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

func (snap *Snapshot) Compare(other *Snapshot) []string {
	var diff []string
	if snap.SnapshotId != other.SnapshotId {
		diff = append(diff, "SnapshotId")
	}
	if snap.Costfunc != other.Costfunc {
		diff = append(diff, "Costfunc")
	}
	diff = append(diff, snap.AllShards.Compare(other.AllShards)...)
	diff = append(diff, snap.AllWorkers.Compare(other.AllWorkers)...)
	diff = append(diff, snap.AllAssignments.Compare(other.AllAssignments)...)
	return diff
}

func (snap *Snapshot) ToJson() map[string]interface{} {
	obj := make(map[string]interface{})
	obj["SnapshotId"] = snap.SnapshotId
	obj["Cost"] = snap.GetCost()
	obj["Shards"] = make([]map[string]interface{}, 0)
	snap.AllShards.VisitAll(func(shardId data.ShardId, shardSnap *ShardSnap) {
		shardObj := shardSnap.ToJson()
		obj["Shards"] = append(obj["Shards"].([]map[string]interface{}), shardObj)
	})
	obj["Workers"] = make([]map[string]interface{}, 0)
	snap.AllWorkers.VisitAll(func(workerFullId data.WorkerFullId, workerSnap *WorkerSnap) {
		workerObj := workerSnap.ToJson()
		obj["Workers"] = append(obj["Workers"].([]map[string]interface{}), workerObj)
	})
	obj["Assignments"] = make([]map[string]interface{}, 0)
	snap.AllAssignments.VisitAll(func(assignmentId data.AssignmentId, assignmentSnap *AssignmentSnap) {
		assignmentObj := assignmentSnap.ToJson()
		obj["Assignments"] = append(obj["Assignments"].([]map[string]interface{}), assignmentObj)
	})
	return obj
}

func (snap *Snapshot) ToJsonString() string {
	obj := snap.ToJson()
	jsonStr, err := json.Marshal(obj)
	if err != nil {
		klogging.Fatal(context.Background()).WithError(err).Log("ToJsonString", "error")
	}
	return string(jsonStr)
}
