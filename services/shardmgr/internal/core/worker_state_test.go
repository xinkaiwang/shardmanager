package core

// func TestWorkerState_ToPilotNode(t *testing.T) {
// 	// 创建测试上下文
// 	ctx := context.Background()

// 	// 创建测试用WorkerState
// 	workerId := data.WorkerId("worker-1")
// 	sessionId := data.SessionId("session-1")
// 	workerState := NewWorkerState(workerId, sessionId)

// 	// 设置通知原因
// 	workerState.NotifyReason = "测试更新"

// 	// 添加分配任务
// 	workerState.Assignments["shard-1:0"] = common.Unit{}
// 	workerState.Assignments["shard-2:1"] = common.Unit{}

// 	// 测试1：正常状态（不关闭）
// 	workerState.ShutdownRequesting = false
// 	workerState.ShutdownPermited = false

// 	pilotNode := workerState.ToPilotNode(ctx)

// 	// 验证基本字段
// 	if pilotNode.WorkerId != string(workerId) {
// 		t.Errorf("WorkerId不匹配：期望 %s，得到 %s", workerId, pilotNode.WorkerId)
// 	}

// 	if pilotNode.SessionId != string(sessionId) {
// 		t.Errorf("SessionId不匹配：期望 %s，得到 %s", sessionId, pilotNode.SessionId)
// 	}

// 	if pilotNode.LastUpdateReason != workerState.NotifyReason {
// 		t.Errorf("LastUpdateReason不匹配：期望 %s，得到 %s", workerState.NotifyReason, pilotNode.LastUpdateReason)
// 	}

// 	// 验证时间戳在合理范围内
// 	currentTime := kcommon.GetWallTimeMs()
// 	if pilotNode.LastUpdateAtMs > currentTime || pilotNode.LastUpdateAtMs < currentTime-5000 {
// 		t.Errorf("LastUpdateAtMs不在合理范围内：%d", pilotNode.LastUpdateAtMs)
// 	}

// 	// 验证分配任务数量
// 	if len(pilotNode.Assignments) != len(workerState.Assignments) {
// 		t.Errorf("Assignments数量不匹配：期望 %d，得到 %d", len(workerState.Assignments), len(pilotNode.Assignments))
// 	}

// 	// 验证任务状态
// 	for _, assignment := range pilotNode.Assignments {
// 		if assignment.State != cougarjson.PAS_Active {
// 			t.Errorf("正常状态下任务状态不匹配：期望 %s，得到 %s", cougarjson.PAS_Active, assignment.State)
// 		}

// 		// 验证ShardId和ReplicaIdx提取是否正确
// 		if assignment.AsginmentId == "shard-1:0" {
// 			if assignment.ShardId != "shard-1" || assignment.ReplicaIdx != 0 {
// 				t.Errorf("ShardId或ReplicaIdx解析错误：ShardId=%s, ReplicaIdx=%d", assignment.ShardId, assignment.ReplicaIdx)
// 			}
// 		} else if assignment.AsginmentId == "shard-2:1" {
// 			if assignment.ShardId != "shard-2" || assignment.ReplicaIdx != 1 {
// 				t.Errorf("ShardId或ReplicaIdx解析错误：ShardId=%s, ReplicaIdx=%d", assignment.ShardId, assignment.ReplicaIdx)
// 			}
// 		}
// 	}

// 	// 测试2：请求关闭状态
// 	workerState.ShutdownRequesting = true

// 	pilotNode = workerState.ToPilotNode(ctx)

// 	// 验证任务状态
// 	for _, assignment := range pilotNode.Assignments {
// 		if assignment.State != cougarjson.PAS_Completed {
// 			t.Errorf("请求关闭状态下任务状态不匹配：期望 %s，得到 %s", cougarjson.PAS_Completed, assignment.State)
// 		}
// 	}

// 	// 测试3：允许关闭状态
// 	workerState.ShutdownRequesting = false
// 	workerState.ShutdownPermited = true

// 	pilotNode = workerState.ToPilotNode(ctx)

// 	// 验证任务状态
// 	for _, assignment := range pilotNode.Assignments {
// 		if assignment.State != cougarjson.PAS_Completed {
// 			t.Errorf("允许关闭状态下任务状态不匹配：期望 %s，得到 %s", cougarjson.PAS_Completed, assignment.State)
// 		}
// 	}
// }
