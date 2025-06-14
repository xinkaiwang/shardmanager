package shadow

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/libs/xklib/krunloop"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

type InitListener interface {
	InitShardState(shardId data.ShardId, shardState *smgjson.ShardStateJson)
	InitWorkerState(workerFullId data.WorkerFullId, workerState *smgjson.WorkerStateJson)
	InitMoveState(proposalId data.ProposalId, proposalState *smgjson.MoveStateJson)
	InitDone()
	StopAndWaitForExit(ctx context.Context)
}

// ShadowState implements StoreProvider/InitListener
type ShadowState struct {
	PathManager *config.PathManager
	AllShards   map[data.ShardId]*smgjson.ShardStateJson
	AllWorkers  map[data.WorkerFullId]*smgjson.WorkerStateJson
	AllExePlans map[data.ProposalId]*smgjson.MoveStateJson
	runloop     *krunloop.RunLoop[*ShadowState]
}

func NewShadowState(ctx context.Context, pm *config.PathManager) *ShadowState {
	shadow := &ShadowState{
		PathManager: pm,
		AllShards:   make(map[data.ShardId]*smgjson.ShardStateJson),
		AllWorkers:  make(map[data.WorkerFullId]*smgjson.WorkerStateJson),
		AllExePlans: make(map[data.ProposalId]*smgjson.MoveStateJson),
	}
	shadow.runloop = krunloop.NewRunLoop(ctx, shadow, "shadow")
	go shadow.runloop.Run(ctx)
	GetCurrentEtcdStore(ctx) // Initialize the etcd store if not already done, this will init metrics and start output metrics
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
func (shadow *ShadowState) InitMoveState(proposalId data.ProposalId, proposalState *smgjson.MoveStateJson) {
	shadow.AllExePlans[proposalId] = proposalState
}

// InitDone is called once when init is done, non-thread-safe operation is not allowed after this
func (shadow *ShadowState) InitDone() {
}

// StoreShardState: StoreProvider interface methods
func (shadow *ShadowState) StoreShardState(shardId data.ShardId, shardState *smgjson.ShardStateJson) {
	eve := NewShardStateJsonEvent(shardId, shardState)
	shadow.runloop.PostEvent(eve)
}

// StoreWorkerState: StoreProvider interface methods
func (shadow *ShadowState) StoreWorkerState(workerFullId data.WorkerFullId, workerState *smgjson.WorkerStateJson) {
	eve := NewWorkerStateJsonEvent(workerFullId, workerState)
	shadow.runloop.PostEvent(eve)
}

// StoreMoveState: StoreProvider interface methods
func (shadow *ShadowState) StoreMoveState(proposalId data.ProposalId, proposalState *smgjson.MoveStateJson) {
	eve := NewExecutionPlanJsonEvent(proposalId, proposalState)
	shadow.runloop.PostEvent(eve)
}

// Visit: StoreProvider interface methods
func (shadow *ShadowState) Visit(visitor func(shadowState *ShadowState)) {
	eve := NewShadowVisitEvent(visitor)
	shadow.runloop.PostEvent(eve)
}

// ShadowVisitEvent implements krunloop.IEvent[*ShadowState]
type ShadowVisitEvent struct {
	createTimeMs int64 // time when the event was created
	Visitor      func(shadowState *ShadowState)
}

func NewShadowVisitEvent(visitor func(shadowState *ShadowState)) *ShadowVisitEvent {
	return &ShadowVisitEvent{
		createTimeMs: kcommon.GetWallTimeMs(),
		Visitor:      visitor,
	}
}
func (eve *ShadowVisitEvent) GetCreateTimeMs() int64 {
	return eve.createTimeMs
}
func (eve *ShadowVisitEvent) GetName() string {
	return "ShadowVisitEvent"
}
func (eve *ShadowVisitEvent) Process(ctx context.Context, shadow *ShadowState) {
	if eve.Visitor != nil {
		eve.Visitor(shadow)
	}
}

func (shadow *ShadowState) StopAndWaitForExit(ctx context.Context) {
	if shadow.runloop != nil {
		shadow.runloop.StopAndWaitForExit()
	}
}

// ShardStateJsonEvent implements krunloop.IEvent[*ShadowState]
type ShardStateJsonEvent struct {
	createTimeMs int64 // time when the event was created
	ShardId      data.ShardId
	ShardState   *smgjson.ShardStateJson
}

func NewShardStateJsonEvent(shardId data.ShardId, shardState *smgjson.ShardStateJson) *ShardStateJsonEvent {
	return &ShardStateJsonEvent{
		createTimeMs: kcommon.GetWallTimeMs(),
		ShardId:      shardId,
		ShardState:   shardState,
	}
}

func (eve *ShardStateJsonEvent) GetCreateTimeMs() int64 {
	return eve.createTimeMs
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
	// klogging.Info(ctx).With("shardId", eve.ShardId).With("lameDuck", eve.ShardState.LameDuck).Log("ShardStateJsonEventUpdate", "正在更新分片状态")
	shadow.AllShards[eve.ShardState.ShardName] = eve.ShardState
	// write to etcd
	key := shadow.PathManager.FmtShardStatePath(eve.ShardId)
	value := eve.ShardState.ToJson()
	klogging.Debug(ctx).With("key", key).With("valueLength", len(value)).Log("ShardStateJsonEventUpdate", "向etcd写入数据")
	GetCurrentEtcdStore(ctx).Put(ctx, key, value, "ShardState")
}

// WorkerStateJsonEvent implements krunloop.IEvent[*ShadowState]
type WorkerStateJsonEvent struct {
	createTimeMs int64 // time when the event was created
	WorkerFullId data.WorkerFullId
	WorkerState  *smgjson.WorkerStateJson
}

func NewWorkerStateJsonEvent(workerFullId data.WorkerFullId, workerState *smgjson.WorkerStateJson) *WorkerStateJsonEvent {
	return &WorkerStateJsonEvent{
		createTimeMs: kcommon.GetWallTimeMs(),
		WorkerFullId: workerFullId,
		WorkerState:  workerState,
	}
}
func (eve *WorkerStateJsonEvent) GetCreateTimeMs() int64 {
	return eve.createTimeMs
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
	currentEtcdStore := GetCurrentEtcdStore(ctx)
	currentEtcdStore.Put(ctx, key, value, "WorkerState")
}

// ExecutionPlanJsonEvent implements krunloop.IEvent[*ShadowState]
type ExecutionPlanJsonEvent struct {
	createTimeMs int64 // time when the event was created
	ProposalId   data.ProposalId
	ExtPlan      *smgjson.MoveStateJson
}

func NewExecutionPlanJsonEvent(proposalId data.ProposalId, extPlan *smgjson.MoveStateJson) *ExecutionPlanJsonEvent {
	return &ExecutionPlanJsonEvent{
		createTimeMs: kcommon.GetWallTimeMs(),
		ProposalId:   proposalId,
		ExtPlan:      extPlan,
	}
}
func (eve *ExecutionPlanJsonEvent) GetCreateTimeMs() int64 {
	return eve.createTimeMs
}
func (eve *ExecutionPlanJsonEvent) GetName() string {
	return "ExecutionPlanJsonEvent"
}

func (eve *ExecutionPlanJsonEvent) Process(ctx context.Context, shadow *ShadowState) {
	key := shadow.PathManager.FmtMoveStatePath(eve.ProposalId)
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
