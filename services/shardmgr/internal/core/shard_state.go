package core

import (
	"context"
	"strconv"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

type ShardState struct {
	ShardId            data.ShardId
	TargetReplicaCount int // solver will depending on this to decide how many replicas to add/remove
	Replicas           map[data.ReplicaIdx]*ReplicaState
	Hints              config.ShardConfig
	CustomProperties   map[string]string
	LameDuck           bool   // soft delete. we will hard delete this shard (in housekeeping) once all replicas are all gone
	LastUpdateTimeMs   int64  // last update time in ms
	LastUpdateReason   string // for logging/debugging purpose only
}

func NewShardState(shardId data.ShardId, replicaCount int) *ShardState {
	shard := &ShardState{
		ShardId:  shardId,
		Replicas: make(map[data.ReplicaIdx]*ReplicaState),
	}
	for i := 0; i < replicaCount; i++ {
		replica := NewReplicaState(shardId, data.ReplicaIdx(i))
		shard.Replicas[data.ReplicaIdx(i)] = replica
	}
	return shard
}

func NewShardStateByPlan(shardLine *smgjson.ShardLineJson, defCfg config.ShardConfig) *ShardState {
	// cfg
	if shardLine.Hints != nil {
		if shardLine.Hints.MinReplicaCount != nil {
			defCfg.MinReplicaCount = int(*shardLine.Hints.MinReplicaCount)
		}
		if shardLine.Hints.MaxReplicaCount != nil {
			defCfg.MaxReplicaCount = int(*shardLine.Hints.MaxReplicaCount)
		}
		if shardLine.Hints.MoveType != nil {
			defCfg.MovePolicy = *shardLine.Hints.MoveType
		}
	}
	return &ShardState{
		ShardId:          data.ShardId(shardLine.ShardName),
		Replicas:         make(map[data.ReplicaIdx]*ReplicaState),
		Hints:            defCfg,
		CustomProperties: shardLine.CustomProperties,
		LameDuck:         false,
		LastUpdateReason: "unmarshal",
	}
}

func NewShardStateByJson(ctx context.Context, ss *ServiceState, shardStateJson *smgjson.ShardStateJson) *ShardState {
	shardState := &ShardState{
		ShardId:            shardStateJson.ShardName,
		Replicas:           make(map[data.ReplicaIdx]*ReplicaState),
		Hints:              ss.ServiceConfig.ShardConfig,
		CustomProperties:   shardStateJson.CustomProperties,
		TargetReplicaCount: shardStateJson.TargetReplicaCount,
		LameDuck:           common.BoolFromInt8(shardStateJson.LameDuck),
		LastUpdateReason:   "unmarshal",
	}
	for i, replicaJson := range shardStateJson.Resplicas {
		replica := NewReplicaStateByJson(shardState, replicaJson)
		shardState.Replicas[data.ReplicaIdx(i)] = replica
	}
	return shardState
}

// return 1 if shard state is updated, 0 if not
func (ss *ServiceState) UpdateShardStateByPlan(shard *ShardState, shardLine *smgjson.ShardLineJson) *DirtyFlag {
	dirtyFlag := NewDirtyFlag()
	newHints := ss.ServiceConfig.ShardConfig
	if shardLine.Hints != nil {
		if shardLine.Hints.MinReplicaCount != nil {
			newHints.MinReplicaCount = int(*shardLine.Hints.MinReplicaCount)
		}
		if shardLine.Hints.MaxReplicaCount != nil {
			newHints.MaxReplicaCount = int(*shardLine.Hints.MaxReplicaCount)
		}
		if shardLine.Hints.MoveType != nil {
			newHints.MovePolicy = *shardLine.Hints.MoveType
		}
	}
	if shard.Hints != newHints {
		shard.Hints = newHints
		// in theory, hints update don't need to trigger flush, cause hints is not part of the shardStateJson.
		// dirtyFlag.AddDirtyFlag("hints")
	}
	dirtyFlag.AddDirtyFlags(shard.reEvaluateReplicaCount())

	if shard.LameDuck { // 如果分片已经被标记为软删除，则undo软删除, 重新激活分片
		shard.LameDuck = false
		dirtyFlag.AddDirtyFlag("undoLameDuck")
	}
	return dirtyFlag
}

func (shard *ShardState) MarkAsSoftDelete(ctx context.Context) {
	shard.LameDuck = true
	shard.TargetReplicaCount = 0
	for _, replica := range shard.Replicas {
		replica.MarkAsSoftDelete(ctx)
	}
}

func (ss *ServiceState) FlushShardState(ctx context.Context, updated []data.ShardId, inserted []data.ShardId, deleted []data.ShardId) {
	// klogging.Info(ctx).With("updated", updated).With("inserted", inserted).With("deleted", deleted).Log("FlushShardState", "开始刷新分片状态")

	for _, shardId := range updated {
		klogging.Info(ctx).With("shardId", shardId).With("time", kcommon.GetWallTimeMs()).Log("FlushShardState", "更新分片")
		ss.storeProvider.StoreShardState(shardId, ss.AllShards[shardId].ToJson())
	}
	for _, shardId := range inserted {
		klogging.Info(ctx).With("shardId", shardId).With("time", kcommon.GetWallTimeMs()).Log("FlushShardState", "插入分片")
		ss.storeProvider.StoreShardState(shardId, ss.AllShards[shardId].ToJson())
	}
	for _, shardId := range deleted {
		if shard, ok := ss.AllShards[shardId]; ok {
			// 使用当前分片的状态创建软删除记录
			// 分片已经在MarkAsSoftDelete中设置了LameDuck=true
			ss.storeProvider.StoreShardState(shardId, shard.ToJson())
		} else {
			// 如果分片已经不在内存中，则删除分片状态
			ss.storeProvider.StoreShardState(shardId, nil)
		}
	}
}

func (shard *ShardState) ToSnapshot(ss *ServiceState) *costfunc.ShardSnap {
	obj := costfunc.NewShardSnap(shard.ShardId, 0)
	obj.TargetReplicaCount = shard.TargetReplicaCount
	for _, replica := range shard.Replicas {
		obj.Replicas[replica.ReplicaIdx] = replica.ToSnapshot(ss)
	}
	return obj
}

func (shard *ShardState) ToJson() *smgjson.ShardStateJson {
	obj := smgjson.NewShardStateJson(shard.ShardId)
	for _, replica := range shard.Replicas {
		obj.Resplicas[replica.ReplicaIdx] = replica.ToJson()
	}
	obj.TargetReplicaCount = shard.TargetReplicaCount
	obj.LameDuck = smgjson.Bool2Int8(shard.LameDuck)
	obj.LastUpdateTimeMs = shard.LastUpdateTimeMs
	obj.LastUpdateReason = shard.LastUpdateReason
	return obj
}

// reEvaluateReplicaCount: based on the current replica count and the minimum replica count, add new replicas if needed
// return >0 means replica added, <0 means replica removed, 0 means no change
func (shard *ShardState) reEvaluateReplicaCount() *DirtyFlag {
	// 重新计算副本数
	curReplicaCount := shard.TargetReplicaCount
	dirtyFlag := NewDirtyFlag()
	targetReplicaCount := curReplicaCount
	if targetReplicaCount < shard.Hints.MinReplicaCount {
		diff := shard.Hints.MinReplicaCount - targetReplicaCount
		targetReplicaCount = shard.Hints.MinReplicaCount
		dirtyFlag.AddDirtyFlag("minReplicaCount:+" + strconv.Itoa(diff))
	}
	if targetReplicaCount > shard.Hints.MaxReplicaCount {
		diff := targetReplicaCount - shard.Hints.MaxReplicaCount
		targetReplicaCount = shard.Hints.MaxReplicaCount
		dirtyFlag.AddDirtyFlag("maxReplicaCount:-" + strconv.Itoa(diff))
	}
	// here we only adjust targetReplicaCount, the actual replica/assign will be add/remove by solvers (through proposal)
	shard.TargetReplicaCount = targetReplicaCount
	return dirtyFlag
}

// func (shard *ShardState) ReEvaluateReplicaCount() *DirtyFlag {
// 	// 重新计算副本数
// 	curReplicaCount := 0
// 	dirtyFlag := NewDirtyFlag()
// 	largestReplicaIdx := data.ReplicaIdx(0)
// 	for _, replica := range shard.Replicas {
// 		if !replica.LameDuck {
// 			curReplicaCount++
// 		}
// 		if replica.ReplicaIdx > largestReplicaIdx {
// 			largestReplicaIdx = replica.ReplicaIdx
// 		}
// 	}
// 	// add new replicas if needed
// 	for curReplicaCount < shard.Hints.MinReplicaCount {
// 		// add new replica
// 		// step 1: find the next available replica index
// 		nextReplicaIdx := shard.findNextAvailableReplicaIndex()
// 		// step 2: create a new replica
// 		replica := NewReplicaState(shard.ShardId, nextReplicaIdx)
// 		shard.Replicas[nextReplicaIdx] = replica
// 		curReplicaCount++
// 		dirtyFlag.AddDirtyFlag("addReplica" + strconv.Itoa(int(nextReplicaIdx)))
// 	}
// 	// (soft delete) remove extra replicas if needed
// 	for curReplicaCount > shard.Hints.MaxReplicaCount {
// 		// remove the last replica
// 		if replica, ok := shard.Replicas[largestReplicaIdx]; ok && !replica.LameDuck {
// 			replica.LameDuck = true
// 			curReplicaCount--
// 			dirtyFlag.AddDirtyFlag("lameDuckReplica" + strconv.Itoa(int(largestReplicaIdx)))
// 		}
// 		largestReplicaIdx--
// 	}
// 	shard.TargetReplicaCount = curReplicaCount
// 	return dirtyFlag
// }

func (shard *ShardState) findNextAvailableReplicaIndex() data.ReplicaIdx {
	// find the next available replica index
	i := 0
	for {
		if _, ok := shard.Replicas[data.ReplicaIdx(i)]; ok {
			i++
		} else {
			break
		}
	}
	return data.ReplicaIdx(i)
}
