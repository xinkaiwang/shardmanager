package core

import (
	"context"
	"testing"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/services/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/etcdprov"
)

func TestWorkerState_ToPilotNode(t *testing.T) {
	// 创建测试上下文
	ctx := context.Background()

	// 配置测试环境
	setup := NewFakeTimeTestSetup(t)
	setup.SetupBasicConfig(ctx)

	// 设置 FakeEtcdProvider
	fakeProvider := setup.FakeEtcd
	etcdprov.RunWithEtcdProvider(fakeProvider, func() {
		// 创建测试用 ServiceState
		ss := AssembleSsWithShadowState(ctx, "test-service")

		// 创建测试用 WorkerState
		workerId := data.WorkerId("worker-1")
		sessionId := data.SessionId("session-1")
		workerState := NewWorkerState(ss, workerId, sessionId, data.ST_MEMORY)

		// 设置通知原因
		updateReason := "测试更新"

		// 创建 WorkerFullId
		workerFullId := data.NewWorkerFullId(workerId, sessionId, data.ST_MEMORY)

		// 创建并添加分配任务到 ServiceState
		assignment1 := NewAssignmentState(
			"shard-1:0",
			"shard-1",
			0,
			workerFullId,
		)
		assignment1.ShouldInPilot = true
		assignment2 := NewAssignmentState(
			"shard-2:1",
			"shard-2",
			1,
			workerFullId,
		)
		assignment2.ShouldInPilot = true
		assignment1.TargetState = cougarjson.CAS_Ready
		assignment2.TargetState = cougarjson.CAS_Ready
		ss.AllAssignments["shard-1:0"] = assignment1
		ss.AllAssignments["shard-2:1"] = assignment2

		// 添加分配任务到 WorkerState
		workerState.Assignments["shard-1:0"] = common.Unit{}
		workerState.Assignments["shard-2:1"] = common.Unit{}

		// 测试1：正常状态（不关闭）
		workerState.ShutdownRequesting = false

		pilotNode := workerState.ToPilotNode(ctx, ss, updateReason)

		// 验证基本字段
		if pilotNode.WorkerId != string(workerId) {
			t.Errorf("WorkerId不匹配：期望 %s，得到 %s", workerId, pilotNode.WorkerId)
		}

		if pilotNode.SessionId != string(sessionId) {
			t.Errorf("SessionId不匹配：期望 %s，得到 %s", sessionId, pilotNode.SessionId)
		}

		if pilotNode.LastUpdateReason != updateReason {
			t.Errorf("LastUpdateReason不匹配：期望 %s，得到 %s", updateReason, pilotNode.LastUpdateReason)
		}

		// 验证时间戳在合理范围内
		currentTime := kcommon.GetWallTimeMs()
		if pilotNode.LastUpdateAtMs > currentTime || pilotNode.LastUpdateAtMs < currentTime-5000 {
			t.Errorf("LastUpdateAtMs不在合理范围内：%d", pilotNode.LastUpdateAtMs)
		}

		// 验证分配任务数量
		if len(pilotNode.Assignments) != len(workerState.Assignments) {
			t.Errorf("Assignments数量不匹配：期望 %d，得到 %d", len(workerState.Assignments), len(pilotNode.Assignments))
		}

		// 验证任务状态
		for _, assignment := range pilotNode.Assignments {
			if assignment.State != cougarjson.CAS_Ready {
				t.Errorf("正常状态下任务状态不匹配：期望 %s，得到 %s", cougarjson.CAS_Ready, assignment.State)
			}

			// 验证ShardId和ReplicaIdx提取是否正确
			if assignment.AsginmentId == "shard-1:0" {
				if assignment.ShardId != "shard-1" || assignment.ReplicaIdx != 0 {
					t.Errorf("ShardId或ReplicaIdx解析错误：ShardId=%s, ReplicaIdx=%d", assignment.ShardId, assignment.ReplicaIdx)
				}
			} else if assignment.AsginmentId == "shard-2:1" {
				if assignment.ShardId != "shard-2" || assignment.ReplicaIdx != 1 {
					t.Errorf("ShardId或ReplicaIdx解析错误：ShardId=%s, ReplicaIdx=%d", assignment.ShardId, assignment.ReplicaIdx)
				}
			}
		}

		// 测试2：请求关闭状态
		workerState.ShutdownRequesting = true
		// 更新任务状态为 ASE_Dropped
		ss.AllAssignments["shard-1:0"].TargetState = cougarjson.CAS_Dropped
		ss.AllAssignments["shard-2:1"].TargetState = cougarjson.CAS_Dropped

		pilotNode = workerState.ToPilotNode(ctx, ss, updateReason)

		// 验证任务状态
		for _, assignment := range pilotNode.Assignments {
			if assignment.State != cougarjson.CAS_Dropped {
				t.Errorf("请求关闭状态下任务状态不匹配：期望 %s，得到 %s", cougarjson.CAS_Dropped, assignment.State)
			}
		}

		// 测试3：允许关闭状态
		workerState.ShutdownRequesting = false
		workerState.State = data.WS_Online_shutdown_permit

		pilotNode = workerState.ToPilotNode(ctx, ss, updateReason)

		// 验证任务状态
		for _, assignment := range pilotNode.Assignments {
			if assignment.State != cougarjson.CAS_Dropped {
				t.Errorf("允许关闭状态下任务状态不匹配：期望 %s，得到 %s", cougarjson.CAS_Dropped, assignment.State)
			}
		}
	})
}
