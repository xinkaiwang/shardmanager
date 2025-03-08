package shadow

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/libs/xklib/krunloop"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

// ShadowState implements StoreProvider
type ShadowState struct {
	PathManager *config.PathManager
	AllShards   map[data.ShardId]*smgjson.ShardStateJson
	AllWorkers  map[data.WorkerFullId]*smgjson.WorkerStateJson
	AllExePlans map[data.ProposalId]*smgjson.ExecutionPlanJson
	runloop     *krunloop.RunLoop[*ShadowState]
}

func NewShadowState(ctx context.Context, pm *config.PathManager) *ShadowState {
	shadow := &ShadowState{
		PathManager: pm,
		AllShards:   make(map[data.ShardId]*smgjson.ShardStateJson),
		AllWorkers:  make(map[data.WorkerFullId]*smgjson.WorkerStateJson),
		AllExePlans: make(map[data.ProposalId]*smgjson.ExecutionPlanJson),
	}
	shadow.runloop = krunloop.NewRunLoop(ctx, shadow, "shadow")
	go shadow.runloop.Run(ctx)
	return shadow
}

func (shadow *ShadowState) IsResource() {}

// InitShardState is only called once when the service starts, that's why it doesn't need to be thread-safe
func (shadow *ShadowState) InitShardState(shardId data.ShardId, shardState *smgjson.ShardStateJson) {
	shadow.AllShards[shardId] = shardState
}

// InitWorkerState is only called once when the service starts, that's why it doesn't need to be thread-safe
func (shadow *ShadowState) InitWorkerState(workerFullId data.WorkerFullId, workerState *smgjson.WorkerStateJson) {
	shadow.AllWorkers[workerFullId] = workerState
}

// InitProposalState is only called once when the service starts, that's why it doesn't need to be thread-safe
func (shadow *ShadowState) InitProposalState(proposalId data.ProposalId, proposalState *smgjson.ExecutionPlanJson) {
	shadow.AllExePlans[proposalId] = proposalState
}

// InitDone is called once when init is done, non-thread-safe operation is not allowed after this
func (shadow *ShadowState) InitDone() {
}

func (shadow *ShadowState) StoreShardState(shardId data.ShardId, shardState *smgjson.ShardStateJson) {
	eve := &ShardStateJsonEvent{
		ShardId:    shardId,
		ShardState: shardState,
	}
	shadow.runloop.EnqueueEvent(eve)
}

func (shadow *ShadowState) StoreWorkerState(workerFullId data.WorkerFullId, workerState *smgjson.WorkerStateJson) {
	eve := &WorkerStateJsonEvent{
		WorkerFullId: workerFullId,
		WorkerState:  workerState,
	}
	shadow.runloop.EnqueueEvent(eve)
}

func (shadow *ShadowState) StoreProposalState(proposalId data.ProposalId, proposalState *smgjson.ExecutionPlanJson) {
	eve := &ExecutionPlanJsonEvent{
		ProposalId: proposalId,
		ExtPlan:    proposalState,
	}
	shadow.runloop.EnqueueEvent(eve)
}

// ShardStateJsonEvent implements krunloop.IEvent[*ShadowState]
type ShardStateJsonEvent struct {
	ShardId    data.ShardId
	ShardState *smgjson.ShardStateJson
}

func (eve *ShardStateJsonEvent) GetName() string {
	return "ShardStateJsonEvent"
}
func (eve *ShardStateJsonEvent) Process(ctx context.Context, shadow *ShadowState) {
	if eve.ShardState == nil {
		klogging.Info(ctx).With("shardId", eve.ShardId).Log("ShardStateJsonEventDelete", "正在删除分片状态")
		delete(shadow.AllShards, eve.ShardId)
		key := shadow.PathManager.FmtShardStatePath(eve.ShardId)
		klogging.Info(ctx).With("key", key).Log("ShardStateJsonEventDelete", "从etcd中删除键")
		GetCurrentEtcdStore(ctx).Put(ctx, key, "", "ShardState")
		return
	}
	klogging.Info(ctx).With("shardId", eve.ShardId).With("lameDuck", eve.ShardState.LameDuck).Log("ShardStateJsonEventUpdate", "正在更新分片状态")
	shadow.AllShards[eve.ShardState.ShardName] = eve.ShardState
	// write to etcd
	key := shadow.PathManager.FmtShardStatePath(eve.ShardId)
	value := eve.ShardState.ToJson()
	klogging.Info(ctx).With("key", key).With("valueLength", len(value)).Log("ShardStateJsonEventUpdate", "向etcd写入数据")
	GetCurrentEtcdStore(ctx).Put(ctx, key, value, "ShardState")
}

// WorkerStateJsonEvent implements krunloop.IEvent[*ShadowState]
type WorkerStateJsonEvent struct {
	WorkerFullId data.WorkerFullId
	WorkerState  *smgjson.WorkerStateJson
}

func (eve *WorkerStateJsonEvent) GetName() string {
	return "WorkerStateJsonEvent"
}
func (eve *WorkerStateJsonEvent) Process(ctx context.Context, shadow *ShadowState) {
	key := shadow.PathManager.FmtWorkerStatePath(eve.WorkerFullId)
	if eve.WorkerState == nil {
		delete(shadow.AllWorkers, eve.WorkerFullId)
		GetCurrentEtcdStore(ctx).Put(ctx, key, "", "WorkerState")
		return
	}
	shadow.AllWorkers[eve.WorkerFullId] = eve.WorkerState
	// write to etcd
	value := eve.WorkerState.ToJson()
	GetCurrentEtcdStore(ctx).Put(ctx, key, value, "WorkerState")
}

// ExecutionPlanJsonEvent implements krunloop.IEvent[*ShadowState]
type ExecutionPlanJsonEvent struct {
	ProposalId data.ProposalId
	ExtPlan    *smgjson.ExecutionPlanJson
}

func (eve *ExecutionPlanJsonEvent) GetName() string {
	return "ProposalStateJsonEvent"
}
func (eve *ExecutionPlanJsonEvent) Process(ctx context.Context, shadow *ShadowState) {
	key := shadow.PathManager.FmtExecutionPlanPath(eve.ProposalId)
	if eve.ExtPlan == nil {
		delete(shadow.AllExePlans, eve.ProposalId)
		GetCurrentEtcdStore(ctx).Put(ctx, key, "", "ExecutionPlan")
		return
	}
	shadow.AllExePlans[eve.ProposalId] = eve.ExtPlan
	// write to etcd
	value := eve.ExtPlan.ToJson()
	GetCurrentEtcdStore(ctx).Put(ctx, key, value, "ExecutionPlan")
}
