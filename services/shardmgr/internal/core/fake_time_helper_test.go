package core

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xinkaiwang/shardmanager/libs/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/libs/unicorn/unicornjson"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/etcdprov"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/shadow"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

// 确保全局状态在每个测试之前被正确重置
func resetGlobalState(_ *testing.T) {
	shadow.ResetEtcdStore()
	etcdprov.ResetEtcdProvider()
}

// Cleanup 清理测试资源，关闭所有相关goroutine
func (setup *FakeTimeTestSetup) Cleanup() {
	// 关闭ServiceState及其相关资源
	if setup.ServiceState != nil {
		klogging.Info(context.Background()).Log("TestCleanup", "关闭ServiceState和相关资源")

		// 停止ServiceState
		setup.ServiceState.StopAndWaitForExit(context.Background())
	}

	// 确保所有的etcd相关资源都被释放
	shadow.ResetEtcdStore()
	etcdprov.ResetEtcdProvider()
	klogging.TimeProvider = nil

	klogging.Info(context.Background()).Log("TestCleanup", "清理完成")
}

/******************************* SnapshotListener *******************************/
// FakeSnapshotListener implements solver.SnapshotListener
type FakeSnapshotListener struct {
	CallCount int
	snapshot  *costfunc.Snapshot
	Reason    string
}

func NewFakeSnapshotListener() *FakeSnapshotListener {
	return &FakeSnapshotListener{}
}
func (l *FakeSnapshotListener) OnSnapshot(ctx context.Context, snapshot *costfunc.Snapshot, reason string) {
	l.CallCount++
	l.snapshot = snapshot
	l.Reason = reason
}

/******************************* FakeTimeTestSetup *******************************/

// FakeTimeTestSetup 包含服务状态测试所需的基本设置
type FakeTimeTestSetup struct {
	ctx                  context.Context
	ServiceState         *ServiceState
	FakeEtcd             *etcdprov.FakeEtcdProvider
	FakeStore            *shadow.FakeEtcdStore
	FakeTime             *kcommon.FakeTimeProvider
	FakeSnapshotListener *FakeSnapshotListener
	PathManager          *config.PathManager
	StatefulType         data.StatefulType
}

// NewFakeTimeTestSetup 创建基本的测试环境
func NewFakeTimeTestSetup(t *testing.T) *FakeTimeTestSetup {
	resetGlobalState(t)

	fakeEtcd := etcdprov.NewFakeEtcdProvider()
	fakeStore := shadow.NewFakeEtcdStore()
	fakeTime := kcommon.NewFakeTimeProvider(1234500000000)
	klogging.TimeProvider = func() time.Time {
		return time.Unix(0, fakeTime.GetWallTimeMs()*1000000)
	}

	setup := &FakeTimeTestSetup{
		ctx:                  context.Background(),
		FakeEtcd:             fakeEtcd,
		FakeStore:            fakeStore,
		FakeTime:             fakeTime,
		FakeSnapshotListener: NewFakeSnapshotListener(),
		PathManager:          config.NewPathManager(),
		StatefulType:         data.ST_MEMORY,
	}

	return setup
}

func (setup *FakeTimeTestSetup) SetupBasicConfig(ctx context.Context, options ...config.ServiceConfigOption) {
	cfg := config.CreateTestServiceConfig(options...)
	setup.FakeEtcd.Set(ctx, "/smg/config/service_config.json", cfg.ToJsonObj().ToJson())
}

func (setup *FakeTimeTestSetup) UpdateServiceConfig(options ...config.ServiceConfigOption) {
	item := setup.FakeEtcd.Get(setup.ctx, "/smg/config/service_config.json")
	serviceConfigJson := smgjson.ParseServiceConfigFromJson(item.Value)
	cfg := config.ServiceConfigFromJson(serviceConfigJson)
	for _, opt := range options {
		opt(cfg)
	}
	setup.FakeEtcd.Set(setup.ctx, "/smg/config/service_config.json", cfg.ToJsonObj().ToJson())
}

func (setup *FakeTimeTestSetup) SetShardPlan(ctx context.Context, shardPlan []string) {
	shardPlanStr := strings.Join(shardPlan, "\n")
	setup.FakeEtcd.Set(ctx, "/smg/config/shard_plan.txt", shardPlanStr)
}

func (setup *FakeTimeTestSetup) ModifyWorkerState(ctx context.Context, workerFullId data.WorkerFullId, fn func(*smgjson.WorkerStateJson) *smgjson.WorkerStateJson) {
	path := setup.PathManager.FmtWorkerStatePath(workerFullId)
	item := setup.FakeEtcd.Get(ctx, path)
	var wsj *smgjson.WorkerStateJson
	if item.Value != "" {
		wsj = smgjson.WorkerStateJsonFromJson(item.Value)
	}
	newWsj := fn(wsj)
	setup.FakeEtcd.Set(ctx, path, newWsj.ToJson())
}

func (setup *FakeTimeTestSetup) ModifyRoutingEntry(ctx context.Context, workerFullId data.WorkerFullId, fn func(*unicornjson.WorkerEntryJson) *unicornjson.WorkerEntryJson) {
	path := setup.PathManager.FmtRoutingPath(workerFullId)
	item := setup.FakeEtcd.Get(ctx, path)
	var entry *unicornjson.WorkerEntryJson
	if item.Value != "" {
		entry = unicornjson.WorkerEntryJsonFromJson(item.Value)
	}
	newEntry := fn(entry)
	if newEntry != nil {
		setup.FakeEtcd.Set(ctx, path, newEntry.ToJson())
	} else {
		setup.FakeEtcd.Delete(ctx, path, false)
	}
}

func (setup *FakeTimeTestSetup) ModifyPilotNode(ctx context.Context, workerFullId data.WorkerFullId, fn func(*cougarjson.PilotNodeJson) *cougarjson.PilotNodeJson) {
	path := setup.PathManager.FmtPilotPath(workerFullId)
	item := setup.FakeEtcd.Get(ctx, path)
	var pilotNode *cougarjson.PilotNodeJson
	if item.Value != "" {
		pilotNode = cougarjson.ParsePilotNodeJson(item.Value)
	}
	newPilotNode := fn(pilotNode)
	if newPilotNode != nil {
		setup.FakeEtcd.Set(ctx, path, newPilotNode.ToJson())
	} else {
		setup.FakeEtcd.Delete(ctx, path, false)
	}
}

func (setup *FakeTimeTestSetup) ModifyShardState(ctx context.Context, shardId data.ShardId, fn func(*smgjson.ShardStateJson) *smgjson.ShardStateJson) {
	path := setup.PathManager.FmtShardStatePath(shardId)
	item := setup.FakeEtcd.Get(ctx, path)
	var shardState *smgjson.ShardStateJson
	if item.Value != "" {
		shardState = smgjson.ShardStateJsonFromJson(item.Value)
	}
	newShardState := fn(shardState)
	setup.FakeEtcd.Set(ctx, path, newShardState.ToJson())
}

// func (setup *FakeTimeTestSetup) UpdateEphNode(ctx context.Context, workerFullId data.WorkerFullId, fn func(*cougarjson.WorkerEphJson) *cougarjson.WorkerEphJson) {
// 	path := setup.ServiceState.PathManager.FmtWorkerEphPath(workerFullId)
// 	item := setup.FakeEtcd.Get(ctx, path)
// 	if item.Value == "" {
// 		fn(nil)
// 		return
// 	}
// 	ephNode := cougarjson.WorkerEphJsonFromJson(item.Value)
// 	newEphNode := fn(ephNode)
// 	setup.FakeEtcd.Set(ctx, path, newEphNode.ToJson())
// }

func (setup *FakeTimeTestSetup) RunWith(fn func()) {
	// 确保测试结束时清理资源
	defer setup.Cleanup()

	etcdprov.RunWithEtcdProvider(setup.FakeEtcd, func() {
		shadow.RunWithEtcdStore(setup.FakeStore, func() {
			kcommon.RunWithTimeProvider(setup.FakeTime, func() {
				fn()
				setup.FakeTime.VirtualTimeForward(setup.ctx, 1000)
			})
		})
	})
}

func (setup *FakeTimeTestSetup) GetPilotNode(workerFullId data.WorkerFullId) *cougarjson.PilotNodeJson {
	path := setup.ServiceState.PathManager.FmtPilotPath(workerFullId)
	item := setup.FakeStore.GetByKey(path)
	if item == "" {
		return nil
	}
	pilotNode := cougarjson.ParsePilotNodeJson(item)
	return pilotNode
}

func (setup *FakeTimeTestSetup) GetEphNode(workerFullId data.WorkerFullId) *cougarjson.WorkerEphJson {
	path := setup.ServiceState.PathManager.FmtWorkerEphPath(workerFullId)
	item := setup.FakeEtcd.Get(setup.ctx, path)
	if item.Value == "" {
		return nil
	}
	ephNode := cougarjson.WorkerEphJsonFromJson(item.Value)
	return ephNode
}

func (setup *FakeTimeTestSetup) GetRoutingEntry(workerFullId data.WorkerFullId) *unicornjson.WorkerEntryJson {
	path := setup.ServiceState.PathManager.FmtRoutingPath(workerFullId)
	item := setup.FakeStore.GetByKey(path)
	if item == "" {
		return nil
	}
	entry := unicornjson.WorkerEntryJsonFromJson(item)
	return entry
}

func (setup *FakeTimeTestSetup) PrintAll(ctx context.Context) {
	setup.FakeStore.PrintAll(ctx)
	setup.FakeEtcd.PrintAll(ctx)
}

/******************************* safeAccessServiceState *******************************/

func (setup *FakeTimeTestSetup) safeAccessServiceState(fn func(*ServiceState)) {
	// 创建同步通道
	completed := make(chan struct{})
	// 创建一个事件并加入队列
	setup.ServiceState.PostEvent(NewServiceStateAccessEvent(func(ss *ServiceState) {
		fn(ss)
		close(completed)
	}))
	// 等待事件处理完成
	<-completed
}

// /******************************* safeAccessServiceState *******************************/
// // 安全地访问 ServiceState 内部状态（同步方式）
// func safeAccessServiceState(ss *ServiceState, fn func(*ServiceState)) {
// 	// 创建同步通道
// 	completed := make(chan struct{})

// 	// 创建一个事件并加入队列
// 	ss.PostEvent(NewServiceStateAccessEvent(func(ss *ServiceState) {
// 		fn(ss)
// 		close(completed)
// 	}))

// 	// 等待事件处理完成
// 	<-completed
// }

// serviceStateAccessEvent implements krunloop.IEvent interface
type serviceStateAccessEvent struct {
	createTimeMs int64
	callback     func(*ServiceState)
}

func NewServiceStateAccessEvent(callback func(*ServiceState)) *serviceStateAccessEvent {
	return &serviceStateAccessEvent{
		createTimeMs: kcommon.GetWallTimeMs(),
		callback:     callback,
	}
}

// GetCreateTimeMs 返回事件创建时间
func (e *serviceStateAccessEvent) GetCreateTimeMs() int64 {
	return e.createTimeMs
}

// GetName 返回事件名称
func (e *serviceStateAccessEvent) GetName() string {
	return "ServiceStateAccess"
}

// Process 实现 IEvent 接口
func (e *serviceStateAccessEvent) Process(ctx context.Context, ss *ServiceState) {
	e.callback(ss)
}

func safeGetWorkerStateEnum(ss *ServiceState, workerFullId data.WorkerFullId) data.WorkerStateEnum {
	var state data.WorkerStateEnum
	safeAccessWorkerById(ss, workerFullId, func(worker *WorkerState) {
		if worker == nil {
			state = data.WS_Unknown
			return
		}
		state = worker.State
	})
	return state
}

func safeAccessWorkerById(ss *ServiceState, workerFullId data.WorkerFullId, fn func(*WorkerState)) {
	completed := make(chan struct{})
	ss.PostEvent(&serviceStateAccessEvent{
		callback: func(ss *ServiceState) {
			worker := ss.FindWorkerStateByWorkerFullId(workerFullId)
			fn(worker)
			close(completed)
		},
	})
	<-completed
}

/******************************* CreateAndSetWorkerEph *******************************/

// CreateAndSetWorkerEph 创建worker eph节点并设置到etcd
// 返回workerFullId和ephPath
func (setup *FakeTimeTestSetup) CreateAndSetWorkerEph(t *testing.T, workerId string, sessionId string, addrPort string) (data.WorkerFullId, string) {
	// 创建workerId对象
	workerFullId := data.NewWorkerFullId(data.WorkerId(workerId), data.SessionId(sessionId), setup.StatefulType)

	// 创建worker eph对象
	workerEph := cougarjson.NewWorkerEphJson(workerId, sessionId, 1234567890000, 60)
	workerEph.AddressPort = addrPort
	workerEphJson := workerEph.ToJson()

	// 设置到etcd
	ephPath := setup.PathManager.FmtWorkerEphPath(workerFullId)
	t.Logf("设置worker eph节点到etcd路径: %s", ephPath)
	setup.FakeEtcd.Set(setup.ctx, ephPath, workerEphJson)

	return workerFullId, ephPath
}

/******************************* WaitUntil *******************************/

// WaitUntil 等待条件满足或超时
func WaitUntil(t *testing.T, condition func() (bool, string), maxWaitMs int, intervalMs int) (bool, int64) {
	startMs := kcommon.GetMonoTimeMs()
	maxDuration := maxWaitMs
	intervalDuration := intervalMs
	startTime := kcommon.GetMonoTimeMs()

	for i := 0; i < maxWaitMs/intervalMs; i++ {
		success, debugInfo := condition()
		if success {
			elapsed := kcommon.GetMonoTimeMs() - startTime
			// t.Logf("条件满足，耗时 %v", elapsed)
			klogging.Debug(context.TODO()).With("尝试", i+1).With("最大尝试次数", maxWaitMs/intervalMs).With("已耗时", elapsed).With("最后状态", debugInfo).With("time", kcommon.GetWallTimeMs()).Log("WaitUntil", "条件满足")
			elapsedMs := kcommon.GetMonoTimeMs() - startMs
			return true, elapsedMs
		}

		elapsed := int(kcommon.GetMonoTimeMs() - startTime)
		if elapsed >= maxDuration {
			// t.Logf("等待条件满足超时，已尝试 %d 次，耗时 %v，最后状态: %s", i+1, elapsed, debugInfo)
			klogging.Debug(context.TODO()).With("尝试", i+1).With("最大尝试次数", maxWaitMs/intervalMs).With("已耗时", elapsed).With("最后状态", debugInfo).With("time", kcommon.GetWallTimeMs()).Log("WaitUntil", "等待条件满足超时")
			elapsedMs := kcommon.GetMonoTimeMs() - startMs
			return false, elapsedMs
		}

		// t.Logf("等待条件满足中 (尝试 %d/%d)，已耗时 %v，当前状态: %s",
		// 	i+1, maxWaitMs/intervalMs, elapsed, debugInfo)
		klogging.Debug(context.TODO()).With("尝试", i+1).With("最大尝试次数", maxWaitMs/intervalMs).With("已耗时", elapsed).With("当前状态", debugInfo).With("time", kcommon.GetWallTimeMs()).Log("WaitUntil", "等待条件满足中")
		kcommon.SleepMs(context.Background(), intervalDuration)
	}
	elapsedMs := kcommon.GetMonoTimeMs() - startMs
	return false, elapsedMs
}

func (setup *FakeTimeTestSetup) WaitUntilSs(t *testing.T, fn func(ss *ServiceState) (bool, string), maxWaitMs int, intervalMs int) (bool, int64) {
	ret, elapsedMs := WaitUntil(t, func() (bool, string) {
		var result bool
		var reason string
		setup.safeAccessServiceState(func(ss *ServiceState) {
			result, reason = fn(ss)
		})
		return result, reason
	}, maxWaitMs, intervalMs)
	return ret, elapsedMs
}

func (setup *FakeTimeTestSetup) WaitUntilWorkerState(t *testing.T, workerFullId data.WorkerFullId, fn func(workerState *WorkerState) (bool, string), maxWaitMs int, intervalMs int) (bool, int64) {
	ret, elapsedMs := WaitUntil(t, func() (bool, string) {
		var result bool
		var reason string
		setup.safeAccessServiceState(func(ss *ServiceState) {
			worker := ss.FindWorkerStateByWorkerFullId(workerFullId)

			result, reason = fn(worker)
		})
		return result, reason
	}, maxWaitMs, intervalMs)
	return ret, elapsedMs
}

func (setup *FakeTimeTestSetup) WaitUntilWorkerStateEnum(t *testing.T, workerFullId data.WorkerFullId, expectedState data.WorkerStateEnum, maxWaitMs int, intervalMs int) (bool, int64) {
	return setup.WaitUntilWorkerState(t, workerFullId, func(ws *WorkerState) (bool, string) {
		if ws == nil {
			return false, "worker state is nil"
		}
		if ws.State != expectedState {
			return false, fmt.Sprintf("worker state is %s, expected %s", ws.State, expectedState)
		}
		return true, ""
	}, maxWaitMs, intervalMs)
}

func (setup *FakeTimeTestSetup) WaitUntilWorkerFullState(t *testing.T, workerFullId data.WorkerFullId, fn func(pilot *WorkerState, assigns map[data.AssignmentId]*AssignmentState) (bool, string), maxWaitMs int, intervalMs int) (bool, int64) {
	ret, elapsedMs := WaitUntil(t, func() (bool, string) {
		var result bool
		var reason string
		setup.safeAccessServiceState(func(ss *ServiceState) {
			worker := ss.FindWorkerStateByWorkerFullId(workerFullId)
			dict := map[data.AssignmentId]*AssignmentState{}
			for _, assignId := range worker.Assignments {
				dict[assignId] = ss.AllAssignments[assignId]
			}
			result, reason = fn(worker, dict)
		})
		return result, reason
	}, maxWaitMs, intervalMs)
	return ret, elapsedMs
}

func (setup *FakeTimeTestSetup) WaitUntilShardCount(t *testing.T, expectedShardCount int, maxWaitMs int, intervalMs int) (bool, int64) {
	ret, elapsedMs := WaitUntil(t, func() (bool, string) {
		var count int
		setup.safeAccessServiceState(func(ss *ServiceState) {
			count = len(ss.AllShards)
		})
		return count == expectedShardCount, strconv.Itoa(count)
	}, maxWaitMs, intervalMs)
	return ret, elapsedMs
}

func (setup *FakeTimeTestSetup) WaitUntilWorkerStatePersisted(t *testing.T, workerFullId data.WorkerFullId, fn func(wsj *smgjson.WorkerStateJson) bool, maxWaitMs int, intervalMs int) (bool, int64) {
	ret, elapsedMs := WaitUntil(t, func() (bool, string) {
		path := setup.ServiceState.PathManager.FmtWorkerStatePath(workerFullId)
		item := setup.FakeStore.GetByKey(path)
		if item == "" {
			return fn(nil), ""
		}
		wsj := smgjson.WorkerStateJsonFromJson(item)
		return fn(wsj), ""
	}, maxWaitMs, intervalMs)
	return ret, elapsedMs
}

func (setup *FakeTimeTestSetup) WaitUntilPilotNode(t *testing.T, workerFullId data.WorkerFullId, fn func(pilot *cougarjson.PilotNodeJson) (bool, string), maxWaitMs int, intervalMs int) (bool, int64) {
	ret, elapsedMs := WaitUntil(t, func() (bool, string) {
		path := setup.ServiceState.PathManager.FmtPilotPath(workerFullId)
		item := setup.FakeStore.GetByKey(path)
		if item == "" {
			return fn(nil)
		}
		pilot := cougarjson.ParsePilotNodeJson(item)
		return fn(pilot)
	}, maxWaitMs, intervalMs)
	return ret, elapsedMs
}

func (setup *FakeTimeTestSetup) WaitUntilRoutingState(t *testing.T, workerFullId data.WorkerFullId, fn func(entry *unicornjson.WorkerEntryJson) (bool, string), maxWaitMs int, intervalMs int) (bool, int64) {
	ret, elapsedMs := WaitUntil(t, func() (bool, string) {
		path := setup.ServiceState.PathManager.FmtRoutingPath(workerFullId)
		item := setup.FakeStore.GetByKey(path)
		if item == "" {
			return fn(nil)
		}
		entry := unicornjson.WorkerEntryJsonFromJson(item)
		return fn(entry)
	}, maxWaitMs, intervalMs)
	return ret, elapsedMs
}

func (setup *FakeTimeTestSetup) WaitUntilSnapshot(t *testing.T, fn func(snapshot *costfunc.Snapshot) (bool, string), maxWaitMs int, intervalMs int) (bool, int64) {
	ret, elapsedMs := WaitUntil(t, func() (bool, string) {
		return fn(setup.FakeSnapshotListener.snapshot)
	}, maxWaitMs, intervalMs)
	return ret, elapsedMs
}

func (setup *FakeTimeTestSetup) WaitUntilSnapshotCurrent(t *testing.T, fn func(snapshot *costfunc.Snapshot) (bool, string), maxWaitMs int, intervalMs int) (bool, int64) {
	ret, elapsedMs := WaitUntil(t, func() (bool, string) {
		result := false
		reason := ""
		setup.safeAccessServiceState(func(ss *ServiceState) {
			result, reason = fn(setup.ServiceState.GetSnapshotCurrentForAny())
		})
		return result, reason
	}, maxWaitMs, intervalMs)
	return ret, elapsedMs
}

// verifyAllShards 验证所有分片的状态
func (setup *FakeTimeTestSetup) verifyAllShardState(t *testing.T, expectedStates map[data.ShardId]ExpectedShardState) []string {
	var allPassed []string
	ctx := context.Background()

	// 创建事件来安全访问 ServiceState
	setup.safeAccessServiceState(func(ss *ServiceState) {
		// 首先输出当前所有分片状态，方便调试
		klogging.Info(ctx).With("totalShards", len(ss.AllShards)).With("expectedShards", len(expectedStates)).Log("VerifyShardState", "当前分片状态")
		for shardId, shard := range ss.AllShards {
			klogging.Info(ctx).With("shardId", shardId).With("lameDuck", shard.LameDuck).Log("VerifyShardState", "分片状态")
		}

		// 验证每个期望的分片
		for shardId, expected := range expectedStates {
			shard, ok := ss.AllShards[shardId]
			if !ok {
				errorMsg := fmt.Sprintf("未找到分片 %s", shardId)
				allPassed = append(allPassed, errorMsg)
				continue // 继续检查其他分片
			}

			if shard.LameDuck != expected.LameDuck {
				errorMsg := fmt.Sprintf("分片 %s 的 lameDuck 状态不符合预期，期望: %v，实际: %v",
					shardId, expected.LameDuck, shard.LameDuck)
				allPassed = append(allPassed, errorMsg)
				continue // 继续检查其他分片
			}

			klogging.Info(ctx).With("shardId", shardId).With("lameDuck", shard.LameDuck).Log("VerifyShardState", "分片状态验证通过")
		}
	})

	// 返回验证结果
	return allPassed
}

type ExpectedShardState struct {
	LameDuck bool
}

func verifyAllShardsInStorage(t *testing.T, setup *FakeTimeTestSetup, expectedStates map[data.ShardId]ExpectedShardState) []string {
	var result []string
	for shardName, expected := range expectedStates {
		shardId := data.ShardId(shardName)
		shardStatePath := setup.ServiceState.PathManager.FmtShardStatePath(shardId)
		val := setup.FakeStore.GetByKey(shardStatePath)
		if val == "" {
			result = append(result, fmt.Sprintf("分片状态未持久化, shard=%s", shardName))
			continue
		}

		if val != "" {
			shardState := smgjson.ShardStateJsonFromJson(val)
			if shardState.ShardName != shardId {
				result = append(result, fmt.Sprintf("分片状态持久化错误, shard=%s, shardState=%v", shardName, shardState))
				continue
			}
			if shardState.LameDuck != smgjson.Bool2Int8(expected.LameDuck) {
				result = append(result, fmt.Sprintf("分片状态持久化错误, shard=%s, shardState=%v", shardName, shardState))
				continue
			}
		}
	}
	return result
}

/******************************* UpdateEphNode *******************************/

func (setup *FakeTimeTestSetup) UpdateEphNode(workerFullId data.WorkerFullId, fn func(*cougarjson.WorkerEphJson) *cougarjson.WorkerEphJson) {
	path := setup.PathManager.FmtWorkerEphPath(workerFullId)
	item := setup.FakeEtcd.Get(setup.ctx, path)
	if item.Value == "" {
		newNode := fn(nil)
		if newNode == nil {
			return
		}
		setup.FakeEtcd.Set(setup.ctx, path, newNode.ToJson())
		return
	}
	eph := cougarjson.WorkerEphJsonFromJson(item.Value)
	newNode := fn(eph)
	if newNode == nil {
		setup.FakeEtcd.Delete(setup.ctx, path, false)
		return
	}
	setup.FakeEtcd.Set(setup.ctx, path, newNode.ToJson())
}
