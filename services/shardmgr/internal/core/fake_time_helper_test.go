package core

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/etcdprov"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/shadow"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
	"github.com/xinkaiwang/shardmanager/services/unicorn/unicornjson"
)

// 确保全局状态在每个测试之前被正确重置
func resetGlobalState(_ *testing.T) {
	shadow.ResetEtcdStore()
	etcdprov.ResetEtcdProvider()
}

/******************************* SnapshotListener *******************************/
// FakeSnapshotListener implements solver.SnapshotListener
type FakeSnapshotListener struct {
	CallCount int
	snapshot  *costfunc.Snapshot
}

func NewFakeSnapshotListener() *FakeSnapshotListener {
	return &FakeSnapshotListener{}
}
func (l *FakeSnapshotListener) OnSnapshot(snapshot *costfunc.Snapshot) {
	l.CallCount++
	l.snapshot = snapshot
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

// SetupBasicConfig 设置基本配置
func (setup *FakeTimeTestSetup) SetupBasicConfig(ctx context.Context, options ...smgjson.ServiceConfigOption) {
	// 准备服务信息
	serviceInfo := smgjson.CreateTestServiceInfo(setup.StatefulType)
	setup.FakeEtcd.Set(ctx, "/smg/config/service_info.json", serviceInfo.ToJson())

	// 准备服务配置
	serviceConfig := smgjson.CreateTestServiceConfigWithOptions(options...)
	setup.FakeEtcd.Set(ctx, "/smg/config/service_config.json", serviceConfig.ToJson())
}

func (setup *FakeTimeTestSetup) RunWith(fn func()) {
	etcdprov.RunWithEtcdProvider(setup.FakeEtcd, func() {
		shadow.RunWithEtcdStore(setup.FakeStore, func() {
			kcommon.RunWithTimeProvider(setup.FakeTime, func() {
				fn()
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
// 安全地访问 ServiceState 内部状态（同步方式）
func safeAccessServiceState(ss *ServiceState, fn func(*ServiceState)) {
	// 创建同步通道
	completed := make(chan struct{})

	// 创建一个事件并加入队列
	ss.PostEvent(NewServiceStateAccessEvent(func(ss *ServiceState) {
		fn(ss)
		close(completed)
	}))

	// 等待事件处理完成
	<-completed
}

// serviceStateAccessEvent 是一个用于访问 ServiceState 的事件
type serviceStateAccessEvent struct {
	callback func(*ServiceState)
}

func NewServiceStateAccessEvent(callback func(*ServiceState)) *serviceStateAccessEvent {
	return &serviceStateAccessEvent{
		callback: callback,
	}
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
			worker := ss.AllWorkers[workerFullId]
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

func (setup *FakeTimeTestSetup) WaitUntilWorkerStateEnum(t *testing.T, workerFullId data.WorkerFullId, expectedState data.WorkerStateEnum, maxWaitMs int, intervalMs int) (bool, int64) {
	return setup.WaitUntilWorkerState(t, workerFullId, func(ws *WorkerState) bool {
		return ws != nil && ws.State == expectedState
	}, maxWaitMs, intervalMs)
}

func (setup *FakeTimeTestSetup) WaitUntilWorkerState(t *testing.T, workerFullId data.WorkerFullId, fn func(pilot *WorkerState) bool, maxWaitMs int, intervalMs int) (bool, int64) {
	ret, elapsedMs := WaitUntil(t, func() (bool, string) {
		var result bool
		safeAccessServiceState(setup.ServiceState, func(ss *ServiceState) {
			worker := ss.AllWorkers[workerFullId]
			result = fn(worker)
		})
		return result, ""
	}, maxWaitMs, intervalMs)
	return ret, elapsedMs
}

func (setup *FakeTimeTestSetup) WaitUntilWorkerFullState(t *testing.T, workerFullId data.WorkerFullId, fn func(pilot *WorkerState, assigns map[data.AssignmentId]*AssignmentState) (bool, string), maxWaitMs int, intervalMs int) (bool, int64) {
	ret, elapsedMs := WaitUntil(t, func() (bool, string) {
		var result bool
		var reason string
		safeAccessServiceState(setup.ServiceState, func(ss *ServiceState) {
			worker := ss.AllWorkers[workerFullId]
			dict := map[data.AssignmentId]*AssignmentState{}
			for assignId := range worker.Assignments {
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
		safeAccessServiceState(setup.ServiceState, func(ss *ServiceState) {
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

func (setup *FakeTimeTestSetup) WaitUntilRoutingState(t *testing.T, workerFullId data.WorkerFullId, fn func(entry *unicornjson.WorkerEntryJson) bool, maxWaitMs int, intervalMs int) (bool, int64) {
	ret, elapsedMs := WaitUntil(t, func() (bool, string) {
		path := setup.ServiceState.PathManager.FmtRoutingPath(workerFullId)
		item := setup.FakeStore.GetByKey(path)
		if item == "" {
			return fn(nil), ""
		}
		entry := unicornjson.WorkerEntryJsonFromJson(item)
		return fn(entry), ""
	}, maxWaitMs, intervalMs)
	return ret, elapsedMs
}

func (setup *FakeTimeTestSetup) WaitUntilSnapshot(t *testing.T, fn func(snapshot *costfunc.Snapshot) (bool, string), maxWaitMs int, intervalMs int) (bool, int64) {
	ret, elapsedMs := WaitUntil(t, func() (bool, string) {
		return fn(setup.FakeSnapshotListener.snapshot)
	}, maxWaitMs, intervalMs)
	return ret, elapsedMs
}

// verifyAllShards 验证所有分片的状态
func verifyAllShardState(t *testing.T, ss *ServiceState, expectedStates map[data.ShardId]ExpectedShardState) []string {
	var allPassed []string
	ctx := context.Background()

	// 创建事件来安全访问 ServiceState
	safeAccessServiceState(ss, func(ss *ServiceState) {
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

func (setup *FakeTimeTestSetup) UpdateEphNode(t *testing.T, workerFullId data.WorkerFullId, fn func(*cougarjson.WorkerEphJson) *cougarjson.WorkerEphJson) {
	path := setup.ServiceState.PathManager.FmtWorkerEphPath(workerFullId)
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
		setup.FakeEtcd.Delete(setup.ctx, path)
		return
	}
	setup.FakeEtcd.Set(setup.ctx, path, newNode.ToJson())
}
