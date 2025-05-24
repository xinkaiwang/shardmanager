package costfunc

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// 测试快照数据 - 包含两个分片副本，都在worker-1上
var testSnapshotStr2 = `
{
  "Assignments": [
    {
      "AssignmentId": "02MEXVRE",
      "ReplicaIdx": 0,
      "ShardId": "shard_1",
      "WorkerFullId": {
        "WorkerId": "worker-1",
        "SessionId": "session-1"
      }
    },
    {
      "AssignmentId": "J9UC0IRS",
      "ReplicaIdx": 0,
      "ShardId": "shard_2",
      "WorkerFullId": {
        "WorkerId": "worker-2",
        "SessionId": "session-2"
      }
    }
  ],
  "Cost": {
    "HardScore": 1,
    "SoftScore": 100.00000000000001
  },
  "Shards": [
    {
  	  "TargetReplicaCount": 1,
      "Replicas": [
        {
          "Assignments": [
            "02MEXVRE"
          ],
          "LameDuck": 0,
          "ReplicaIdx": 0,
          "ShardId": "shard_1"
        }
      ],
      "ShardId": "shard_1"
    },
    {
	  "TargetReplicaCount": 1,
      "Replicas": [
        {
          "Assignments": [
            "J9UC0IRS"
          ],
          "LameDuck": 0,
          "ReplicaIdx": 0,
          "ShardId": "shard_2"
        }
      ],
      "ShardId": "shard_2"
    }
  ],
  "SnapshotId": "4JAI9GCS",
  "Workers": [
    {
      "Assignments": [
        {
          "AssignmentId": "02MEXVRE",
          "ShardId": "shard_1"
        }
      ],
      "Draining": 1,
      "WorkerFullId": {
        "WorkerId": "worker-1",
        "SessionId": "session-1"
      }
    },
    {
      "Assignments": [
        {
          "AssignmentId": "J9UC0IRS",
          "ShardId": "shard_2"
        }
      ],
      "Draining": 0,
      "WorkerFullId": {
        "WorkerId": "worker-2",
        "SessionId": "session-2"
      }
    }
  ]
}
`

func TestCalCostFromJsonSnapshot2(t *testing.T) {
	ctx := context.Background()

	// 1. 解析JSON快照
	var jsonData map[string]interface{}
	err := json.Unmarshal([]byte(testSnapshotStr2), &jsonData)
	assert.NoError(t, err, "解析JSON失败")

	// 2. 创建快照对象
	snapshot := NewSnapshot(ctx, createDefaultCostConfig())
	err = snapshotFromJson(snapshot, jsonData)
	assert.NoError(t, err, "从JSON创建快照失败")

	// 3. 计算成本
	cost := snapshot.GetCost()

	// 4. 验证成本
	assert.Equal(t, int32(1), cost.HardScore, "硬成本计算不匹配")
	// assert.Equal(t, 100, cost.SoftScore, "软成本计算不匹配")

	// 5. proposal
	{
		move := MoveParseFromSignature("worker-2:session-2/shard_2:0/worker-1:session-1", snapshot)
		newSnap := snapshot.Clone()
		move.Apply(newSnap, AM_Relaxed)
		cost2 := newSnap.GetCost()
		assert.Equal(t, true, cost2.HardScore > cost.HardScore, "硬成本计算不匹配")
	}

	{
		move := MoveParseFromSignature("worker-1:session-1/shard_1:0/worker-2:session-2", snapshot)
		newSnap := snapshot.Clone()
		move.Apply(newSnap, AM_Strict)
		cost2 := newSnap.GetCost()
		assert.Equal(t, true, cost2.HardScore < cost.HardScore, "硬成本计算不匹配")
	}

	// 5. 输出成本详情进行调试
	t.Logf("计算的成本: HardScore=%d, SoftScore=%.2f", cost.HardScore, cost.SoftScore)

}
