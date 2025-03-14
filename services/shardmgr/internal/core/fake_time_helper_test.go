package core

import (
	"context"
	"testing"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/services/cougar/cougarjson"
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

/******************************* FakeTimeTestSetup *******************************/

// FakeTimeTestSetup 包含服务状态测试所需的基本设置
type FakeTimeTestSetup struct {
	ctx       context.Context
	FakeEtcd  *etcdprov.FakeEtcdProvider
	FakeStore *shadow.FakeEtcdStore
	FakeTime  *kcommon.FakeTimeProvider
}

// NewFakeTimeTestSetup 创建基本的测试环境
func NewFakeTimeTestSetup(t *testing.T) *FakeTimeTestSetup {
	resetGlobalState(t)

	fakeEtcd := etcdprov.NewFakeEtcdProvider()
	fakeStore := shadow.NewFakeEtcdStore()
	fakeTime := kcommon.NewFakeTimeProvider(1234500000000)

	setup := &FakeTimeTestSetup{
		ctx:       context.Background(),
		FakeEtcd:  fakeEtcd,
		FakeStore: fakeStore,
		FakeTime:  fakeTime,
	}

	return setup
}

// SetupBasicConfig 设置基本配置
func (setup *FakeTimeTestSetup) SetupBasicConfig(ctx context.Context, options ...smgjson.ServiceConfigOption) {
	// 准备服务信息
	serviceInfo := smgjson.CreateTestServiceInfo()
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

/******************************* ftCreateAndSetWorkerEph *******************************/

// ftCreateAndSetWorkerEph 创建worker eph节点并设置到etcd
// 返回workerFullId和ephPath
func ftCreateAndSetWorkerEph(t *testing.T, ss *ServiceState, setup *FakeTimeTestSetup, workerId string, sessionId string, addrPort string) (data.WorkerFullId, string) {
	// 创建workerId对象
	workerFullId := data.NewWorkerFullId(data.WorkerId(workerId), data.SessionId(sessionId), ss.IsStateInMemory())

	// 创建worker eph对象
	workerEph := cougarjson.NewWorkerEphJson(workerId, sessionId, 1234567890000, 60)
	workerEph.AddressPort = addrPort
	workerEphJson := workerEph.ToJson()

	// 设置到etcd
	ephPath := ss.PathManager.FmtWorkerEphPath(workerFullId)
	t.Logf("设置worker eph节点到etcd路径: %s", ephPath)
	setup.FakeEtcd.Set(setup.ctx, ephPath, workerEphJson)

	return workerFullId, ephPath
}

/******************************* WaitUntil *******************************/

// WaitUntil 等待条件满足或超时
func WaitUntil(t *testing.T, condition func() (bool, string), maxWaitMs int, intervalMs int) bool {
	maxDuration := maxWaitMs
	intervalDuration := intervalMs
	startTime := kcommon.GetMonoTimeMs()

	for i := 0; i < maxWaitMs/intervalMs; i++ {
		success, debugInfo := condition()
		if success {
			elapsed := kcommon.GetMonoTimeMs() - startTime
			t.Logf("条件满足，耗时 %v", elapsed)
			return true
		}

		elapsed := int(kcommon.GetMonoTimeMs() - startTime)
		if elapsed >= maxDuration {
			t.Logf("等待条件满足超时，已尝试 %d 次，耗时 %v，最后状态: %s", i+1, elapsed, debugInfo)
			return false
		}

		t.Logf("等待条件满足中 (尝试 %d/%d)，已耗时 %v，当前状态: %s",
			i+1, maxWaitMs/intervalMs, elapsed, debugInfo)
		kcommon.SleepMs(context.Background(), intervalDuration)
	}
	return false
}

func WaitUntilWorkerState(t *testing.T, ss *ServiceState, workerFullId data.WorkerFullId, expectedState data.WorkerStateEnum, maxWaitMs int, intervalMs int) (bool, int64) {
	startMs := kcommon.GetMonoTimeMs()
	ret := WaitUntil(t, func() (bool, string) {
		state := safeGetWorkerStateEnum(ss, workerFullId)
		return state == expectedState, string(state)
	}, maxWaitMs, intervalMs)
	elapsedMs := kcommon.GetMonoTimeMs() - startMs
	return ret, elapsedMs
}
