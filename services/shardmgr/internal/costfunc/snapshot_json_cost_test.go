package costfunc

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

// 测试快照数据 - 包含两个分片副本，都在worker-1上
var testSnapshotStr = `
{
  "Assignments": [
    {
      "AssignmentId": "BMNCP4CG",
      "ReplicaIdx": 0,
      "ShardId": "shard_1",
      "WorkerFullId": {
        "WorkerId": "worker-1",
        "SessionId": "session-1"
      }
    },
    {
      "AssignmentId": "0M1TAE9G",
      "ReplicaIdx": 1,
      "ShardId": "shard_1",
      "WorkerFullId": {
        "WorkerId": "worker-1",
        "SessionId": "session-1"
      }
    }
  ],
  "Cost": {
    "HardScore": 0,
    "SoftScore": 50.00000000000001
  },
  "Shards": [
    {
	  "TargetReplicaCount": 1,
      "Replicas": [
        {
          "Assignments": [
            "BMNCP4CG"
          ],
          "LameDuck": 0,
          "ReplicaIdx": 0,
          "ShardId": "shard_1"
        },
        {
          "Assignments": [
            "0M1TAE9G"
          ],
          "LameDuck": 1,
          "ReplicaIdx": 1,
          "ShardId": "shard_1"
        }
      ],
      "ShardId": "shard_1"
    }
  ],
  "SnapshotId": "CF4B8L4D",
  "Workers": [
    {
      "Assignments": [
        
      ],
      "Draining": 0,
      "WorkerFullId": {
        "WorkerId": "worker-2",
        "SessionId": "session-2"
      }
    },
    {
      "Assignments": [
        {
          "AssignmentId": "0M1TAE9G",
          "ShardId": "shard_1"
        },
        {
          "AssignmentId": "BMNCP4CG",
          "ShardId": "shard_1"
        }
      ],
      "Draining": 0,
      "WorkerFullId": {
        "WorkerId": "worker-1",
        "SessionId": "session-1"
      }
    }
  ]
}
`

// 测试快照数据2 - 无分配的情况
var testEmptySnapshotStr = `
{
  "Assignments": [],
  "Cost": {
    "HardScore": 4,
    "SoftScore": 0
  },
  "Shards": [
    {
	  "TargetReplicaCount": 2,
      "Replicas": [
        {
          "Assignments": [],
          "LameDuck": 1,
          "ReplicaIdx": 0,
          "ShardId": "shard_1"
        },
        {
          "Assignments": [],
          "LameDuck": 1,
          "ReplicaIdx": 1,
          "ShardId": "shard_1"
        }
      ],
      "ShardId": "shard_1"
    }
  ],
  "SnapshotId": "EMPTY1234",
  "Workers": [
    {
      "Assignments": [],
      "Draining": 0,
      "WorkerFullId": {
        "WorkerId": "worker-1",
        "SessionId": "session-1"
      }
    },
    {
      "Assignments": [],
      "Draining": 0,
      "WorkerFullId": {
        "WorkerId": "worker-2",
        "SessionId": "session-2"
      }
    }
  ]
}
`

// 测试函数：根据JSON字符串解析快照并计算成本
func TestCalCostFromJsonSnapshot(t *testing.T) {
	ctx := context.Background()

	// 准备测试用例
	tests := []struct {
		name         string
		snapshotJson string
		expectedHard int32
		expectedSoft float64
		withLameDuck bool
	}{
		{
			name:         "默认快照成本计算",
			snapshotJson: testSnapshotStr,
			expectedHard: 11, // H3 违规(10分) + H5 违规(1分，有LameDuck, 每个 assign 1分)
			expectedSoft: 50.00000000000001,
			withLameDuck: true,
		},
		{
			name:         "空分配快照成本计算",
			snapshotJson: testEmptySnapshotStr,
			expectedHard: 4, // 两个无分配的副本，每个2分
			expectedSoft: 0,
			withLameDuck: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 1. 解析JSON快照
			var jsonData map[string]interface{}
			err := json.Unmarshal([]byte(tt.snapshotJson), &jsonData)
			assert.NoError(t, err, "解析JSON失败")

			// 2. 创建快照对象
			snapshot := NewSnapshot(ctx, createDefaultCostConfig())
			err = snapshotFromJson(snapshot, jsonData)
			assert.NoError(t, err, "从JSON创建快照失败")

			// 3. 计算成本
			cost := snapshot.GetCost()

			// 4. 验证成本
			assert.Equal(t, tt.expectedHard, cost.HardScore, "硬成本计算不匹配")
			assert.Equal(t, tt.expectedSoft, cost.SoftScore, "软成本计算不匹配")

			// 5. 输出成本详情进行调试
			t.Logf("计算的成本: HardScore=%d, SoftScore=%.2f", cost.HardScore, cost.SoftScore)
		})
	}
}

// 测试从头创建快照并计算成本
func TestCalCostFromBuildSnapshot(t *testing.T) {
	ctx := context.Background()

	// 创建一个默认的成本函数配置
	costConfig := createDefaultCostConfig()

	// 创建一个新的快照
	snapshot := NewSnapshot(ctx, costConfig)

	// 添加分片和副本
	shard := NewShardSnap("shard_1", 0)
	snapshot.AllShards.Set("shard_1", shard)

	shard.TargetReplicaCount = 2

	// 添加工作节点1
	worker1 := NewWorkerSnap(data.WorkerFullId{WorkerId: "worker-1", SessionId: "session-1"})
	snapshot.AllWorkers.Set(worker1.WorkerFullId, worker1)

	// 添加工作节点2
	worker2 := NewWorkerSnap(data.WorkerFullId{WorkerId: "worker-2", SessionId: "session-2"})
	snapshot.AllWorkers.Set(worker2.WorkerFullId, worker2)

	// 计算初始成本 - 应该是4分（两个未分配的副本，每个2分）
	initialCost := snapshot.GetCost()
	assert.Equal(t, int32(4), initialCost.HardScore, "初始硬成本应为4（两个未分配副本）")
	assert.Equal(t, float64(0), initialCost.SoftScore, "初始软成本应为0")

	// 为副本0分配工作节点
	assignment1 := NewAssignmentSnap("shard_1", 0, "assign-1", worker1.WorkerFullId)
	snapshot = snapshot.Clone()
	snapshot.AllAssignments.Set("assign-1", assignment1)
	replica0 := NewReplicaSnap("shard_1", 0)
	shard.Replicas[0] = replica0
	replica0.Assignments["assign-1"] = common.Unit{}
	worker1.Assignments["shard_1"] = "assign-1"

	// 计算中间成本 - 应该是2分（副本1未分配）
	midCost := snapshot.GetCost()
	assert.Equal(t, int32(2), midCost.HardScore, "中间硬成本应为2（一个未分配副本）")

	// 为副本1分配同一个工作节点 - 违反了硬规则H3
	assignment2 := NewAssignmentSnap("shard_1", 1, "assign-2", worker1.WorkerFullId)
	snapshot = snapshot.Clone()
	snapshot.AllAssignments.Set("assign-2", assignment2)
	// 添加副本1
	replica1 := NewReplicaSnap("shard_1", 1)
	shard.Replicas[1] = replica1
	replica1.Assignments["assign-2"] = common.Unit{}
	worker1.Assignments["shard_1"] = "assign-2" // 这里其实有问题，一个worker不能对同一个shard有多个assignments

	// 计算最终成本 - 应该是10分（违反H3规则）
	finalCost := snapshot.GetCost()
	assert.Equal(t, int32(10), finalCost.HardScore, "最终硬成本应为10（违反H3规则）")

	t.Logf("初始成本: %v", initialCost)
	t.Logf("中间成本: %v", midCost)
	t.Logf("最终成本: %v", finalCost)
}

// 创建默认的成本函数配置
func createDefaultCostConfig() config.CostfuncConfig {
	return config.CostfuncConfig{
		ShardCountCostEnable: true,
		ShardCountCostNorm:   10,
		WorkerMaxAssignments: 20,
	}
}

// 从JSON对象创建快照
func snapshotFromJson(snapshot *Snapshot, jsonData map[string]interface{}) error {
	// 解析SnapshotId
	if snapshotId, ok := jsonData["SnapshotId"].(string); ok {
		snapshot.SnapshotId = SnapshotId(snapshotId)
	}

	// 解析Shards
	if shardsJson, ok := jsonData["Shards"].([]interface{}); ok {
		for _, shardJson := range shardsJson {
			shardMap := shardJson.(map[string]interface{})
			shardId := data.ShardId(shardMap["ShardId"].(string))

			shardSnap := ShardSnapParseFromJson(shardMap)
			snapshot.AllShards.Set(shardId, shardSnap)
		}
	}

	// 解析Workers
	if workersJson, ok := jsonData["Workers"].([]interface{}); ok {
		for _, workerJson := range workersJson {
			workerMap := workerJson.(map[string]interface{})
			workerFullIdMap := workerMap["WorkerFullId"].(map[string]interface{})

			workerId := data.WorkerId(workerFullIdMap["WorkerId"].(string))
			sessionId := data.SessionId(workerFullIdMap["SessionId"].(string))
			workerFullId := data.WorkerFullId{WorkerId: workerId, SessionId: sessionId}

			// 创建工作节点
			workerSnap := NewWorkerSnap(workerFullId)
			snapshot.AllWorkers.Set(workerFullId, workerSnap)

			// 设置Draining状态
			if draining, ok := workerMap["Draining"].(float64); ok {
				workerSnap.Draining = draining > 0
			}

			// 解析分配
			if assignmentsJson, ok := workerMap["Assignments"].([]interface{}); ok {
				for _, assignJson := range assignmentsJson {
					assignMap := assignJson.(map[string]interface{})
					shardId := data.ShardId(assignMap["ShardId"].(string))
					assignmentId := data.AssignmentId(assignMap["AssignmentId"].(string))
					workerSnap.Assignments[shardId] = assignmentId
				}
			}
		}
	}

	// 解析Assignments
	if assignmentsJson, ok := jsonData["Assignments"].([]interface{}); ok {
		for _, assignJson := range assignmentsJson {
			assignMap := assignJson.(map[string]interface{})

			shardId := data.ShardId(assignMap["ShardId"].(string))
			replicaIdx := data.ReplicaIdx(int32(assignMap["ReplicaIdx"].(float64)))
			assignmentId := data.AssignmentId(assignMap["AssignmentId"].(string))

			workerFullIdMap := assignMap["WorkerFullId"].(map[string]interface{})
			workerId := data.WorkerId(workerFullIdMap["WorkerId"].(string))
			sessionId := data.SessionId(workerFullIdMap["SessionId"].(string))
			workerFullId := data.WorkerFullId{WorkerId: workerId, SessionId: sessionId}

			// 创建分配
			assignmentSnap := NewAssignmentSnap(shardId, replicaIdx, assignmentId, workerFullId)
			snapshot.AllAssignments.Set(assignmentId, assignmentSnap)
		}
	}

	return nil
}
