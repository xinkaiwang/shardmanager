package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

func TestWorkerShutdownRequestFull(t *testing.T) {
	ctx := context.Background()
	klogging.SetDefaultLogger(klogging.NewLogrusLogger(ctx).SetConfig(ctx, "debug", "text"))

	// 配置测试环境
	setup := NewFakeTimeTestSetup(t)
	setup.SetupBasicConfig(ctx)
	klogging.Info(ctx).Log("TestWorkerShutdownRequestFull", "测试环境已配置")

	fn := func() {
		// Step 0: 创建 ServiceState
		ss := NewServiceState(ctx, "TestWorkerShutdownRequest")
		setup.ServiceState = ss

		// Step 1: 创建 worker-1 eph
		klogging.Info(ctx).Log("TestWorkerShutdownRequestFull", "创建 worker-1 eph")
		workerFullId := data.NewWorkerFullId("worker-1", "session-1", true)
		setup.UpdateEphNode(t, workerFullId, func(*cougarjson.WorkerEphJson) *cougarjson.WorkerEphJson {
			return cougarjson.NewWorkerEphJson(string(workerFullId.WorkerId), "session-1", 1234567890, 100)
		})

		{
			// 等待worker state创建，状态变为WS_Online_healthy
			waitSucc, elapsedMs := WaitUntilWorkerState(t, ss, workerFullId, func(ws *WorkerState) bool {
				return ws != nil && ws.WorkerId == workerFullId.WorkerId && ws.ShutdownRequesting == false
			}, 1000, 10)
			assert.Equal(t, true, waitSucc, "worker state 已创建 elapsedMs=%d", elapsedMs)
		}

		// Step 2: 更新worker eph节点，设置ReqShutDown=1
		klogging.Info(ctx).Log("TestWorkerShutdownRequestFull", "更新worker eph节点，设置ReqShutDown=1")
		setup.UpdateEphNode(t, workerFullId, func(wej *cougarjson.WorkerEphJson) *cougarjson.WorkerEphJson {
			wej.ReqShutDown = 1
			wej.LastUpdateAtMs = 1234567891
			wej.LastUpdateReason = "ReqShutDown"
			return wej
		})

		// 等待worker状态变为WS_Online_shutdown_req
		{
			waitSucc, elapsedMs := WaitUntilWorkerState(t, ss, workerFullId, func(ws *WorkerState) bool {
				return ws != nil && ws.WorkerId == workerFullId.WorkerId && ws.ShutdownRequesting == true
			}, 1000, 10)
			assert.Equal(t, true, waitSucc, "worker state ShutdownRequesting已设置为true elapsedMs=%d", elapsedMs)
		}

		{
			// 等待worker状态
			waitSucc, elapsedMs := WaitUntilWorkerState(t, ss, workerFullId, func(ws *WorkerState) bool {
				return ws.State == data.WS_Online_shutdown_permit
			}, 10*1000, 1000)
			assert.Equal(t, true, waitSucc, "worker state 状态已设置为 WS_Online_shutdown_permit elapsedMs=%d", elapsedMs)
		}
		{
			// 等待 pilot 状态
			waitSucc, elapsedMs := setup.WaitUntilPilotNode(t, workerFullId, func(pnj *cougarjson.PilotNodeJson) bool {
				return pnj != nil && pnj.ShutdownPermited == 1
			}, 10*1000, 1000)
			assert.Equal(t, true, waitSucc, "pilot node ShutdownPermited已设置为1 elapsedMs=%d", elapsedMs)
		}

		// Step 3: 更新worker eph节点
		klogging.Info(ctx).Log("TestWorkerShutdownRequestFull", "删除worker eph节点")
		setup.UpdateEphNode(t, workerFullId, func(wej *cougarjson.WorkerEphJson) *cougarjson.WorkerEphJson {
			return nil // 删除worker eph节点
		})

		{
			// 等待worker状态
			waitSucc, elapsedMs := WaitUntilWorkerState(t, ss, workerFullId, func(ws *WorkerState) bool {
				return ws == nil // worker state 已删除
			}, 60*1000, 1000)
			assert.Equal(t, true, waitSucc, "worker state 状态已设置为 WS_Online_shutdown_permit elapsedMs=%d", elapsedMs)
		}
		// assert.Equal(t, 0, 1, "测试结束")
	}
	// 使用 FakeTimeProvider 和模拟的 EtcdProvider/EtcdStore 运行测试
	setup.RunWith(fn)
}
