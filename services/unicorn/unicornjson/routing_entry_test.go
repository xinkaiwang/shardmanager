package unicornjson

import (
	"testing"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
)

func TestWorkerEntryJsonMarshal(t *testing.T) {
	tests := []struct {
		name     string
		input    *WorkerEntryJson
		expected string
	}{
		{
			name: "完整字段",
			input: func() *WorkerEntryJson {
				worker := NewWorkerEntryJson(
					"unicorn-worker-75fffc88f9-fkbcm",
					"10.0.0.32:8080",
					10,
					"update",
				)
				worker.Assignments = append(worker.Assignments,
					NewAssignmentJson("shard-1", 1, "asg-1"),
					NewAssignmentJson("shard-2", 0, "asg-2"), // ReplicaIdx 为 0，测试 omitempty
				)
				// 设置固定的时间戳用于测试
				worker.LastUpdateAtMs = 1742104602847
				return worker
			}(),
			expected: `{"worker_id":"unicorn-worker-75fffc88f9-fkbcm","addr_port":"10.0.0.32:8080","capacity":10,"assignments":[{"shd":"shard-1","idx":1,"asg":"asg-1"},{"shd":"shard-2","asg":"asg-2"}],"update_time_ms":1742104602847,"update_reason":"update"}`,
		},
		{
			name: "最小字段",
			input: func() *WorkerEntryJson {
				worker := NewWorkerEntryJson(
					"unicorn-worker-75fffc88f9-xj9n2",
					"10.0.0.33:8080",
					20,
					"update",
				)
				// 设置固定的时间戳用于测试
				worker.LastUpdateAtMs = 1742104602847
				return worker
			}(),
			expected: `{"worker_id":"unicorn-worker-75fffc88f9-xj9n2","addr_port":"10.0.0.33:8080","capacity":20,"update_time_ms":1742104602847,"update_reason":"update"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.input.ToJson()
			if got != tt.expected {
				t.Errorf("ToJson() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestWorkerEntryJsonUnmarshal(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      *WorkerEntryJson
		wantPanic bool
		panicType string
	}{
		{
			name:  "完整字段",
			input: `{"worker_id":"unicorn-worker-75fffc88f9-fkbcm","addr_port":"10.0.0.32:8080","capacity":10,"assignments":[{"shd":"shard-1","idx":1,"asg":"asg-1"},{"shd":"shard-2","asg":"asg-2"}]}`,
			want: func() *WorkerEntryJson {
				worker := NewWorkerEntryJson(
					"unicorn-worker-75fffc88f9-fkbcm",
					"10.0.0.32:8080",
					10,
					"update",
				)
				worker.Assignments = append(worker.Assignments,
					NewAssignmentJson("shard-1", 1, "asg-1"),
					NewAssignmentJson("shard-2", 0, "asg-2"),
				)
				return worker
			}(),
			wantPanic: false,
		},
		{
			name:  "最小字段",
			input: `{"worker_id":"unicorn-worker-75fffc88f9-xj9n2","addr_port":"10.0.0.33:8080","capacity":20}`,
			want: NewWorkerEntryJson(
				"unicorn-worker-75fffc88f9-xj9n2",
				"10.0.0.33:8080",
				20,
				"",
			),
			wantPanic: false,
		},
		{
			name:  "缺少必需字段",
			input: `{"worker_id":"unicorn-worker-75fffc88f9-r8t3v","capacity":40}`,
			want: NewWorkerEntryJson(
				"unicorn-worker-75fffc88f9-r8t3v",
				"",
				40,
				"",
			),
			wantPanic: false,
		},
		{
			name:      "格式错误",
			input:     `{"worker_id":123}`, // worker_id 应该是字符串
			wantPanic: true,
			panicType: "UnmarshalError",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got *WorkerEntryJson
			var panicValue interface{}

			func() {
				defer func() {
					panicValue = recover()
				}()
				got = WorkerEntryJsonFromJson(tt.input)
			}()

			if tt.wantPanic {
				if panicValue == nil {
					t.Error("Expected panic but got none")
					return
				}
				if kerr, ok := panicValue.(*kerror.Kerror); ok {
					if kerr.Type != tt.panicType {
						t.Errorf("Expected panic type %v, got %v", tt.panicType, kerr.Type)
					}
				} else {
					t.Errorf("Expected kerror.Kerror panic, got %T: %v", panicValue, panicValue)
				}
				return
			}

			if panicValue != nil {
				t.Errorf("Unexpected panic: %v", panicValue)
				return
			}

			if got.WorkerId != tt.want.WorkerId {
				t.Errorf("WorkerId = %v, want %v", got.WorkerId, tt.want.WorkerId)
			}
			if got.AddressPort != tt.want.AddressPort {
				t.Errorf("AddressPort = %v, want %v", got.AddressPort, tt.want.AddressPort)
			}
			if got.Capacity != tt.want.Capacity {
				t.Errorf("Capacity = %v, want %v", got.Capacity, tt.want.Capacity)
			}

			// 比较 Assignments 数组
			if len(got.Assignments) != len(tt.want.Assignments) {
				t.Errorf("Assignments length = %v, want %v", len(got.Assignments), len(tt.want.Assignments))
			}
			for i, a := range tt.want.Assignments {
				if got.Assignments[i].ShardId != a.ShardId {
					t.Errorf("Assignments[%d].ShardId = %v, want %v", i, got.Assignments[i].ShardId, a.ShardId)
				}
				if got.Assignments[i].ReplicaIdx != a.ReplicaIdx {
					t.Errorf("Assignments[%d].ReplicaIdx = %v, want %v", i, got.Assignments[i].ReplicaIdx, a.ReplicaIdx)
				}
				if got.Assignments[i].AsginmentId != a.AsginmentId {
					t.Errorf("Assignments[%d].AsginmentId = %v, want %v", i, got.Assignments[i].AsginmentId, a.AsginmentId)
				}
			}
		})
	}
}

// TestWorkerEntryJsonRoundTrip 测试序列化后再反序列化是否保持一致
func TestWorkerEntryJsonRoundTrip(t *testing.T) {
	original := func() *WorkerEntryJson {
		worker := NewWorkerEntryJson(
			"unicorn-worker-75fffc88f9-fkbcm",
			"10.0.0.32:8080",
			10,
			"update",
		)
		worker.Assignments = append(worker.Assignments,
			NewAssignmentJson("shard-1", 1, "asg-1"),
			NewAssignmentJson("shard-2", 0, "asg-2"),
		)
		return worker
	}()

	// 序列化
	jsonStr := original.ToJson()

	// 反序列化
	decoded := WorkerEntryJsonFromJson(jsonStr)

	// 比较结果
	if original.WorkerId != decoded.WorkerId {
		t.Errorf("WorkerId mismatch: got %v, want %v", decoded.WorkerId, original.WorkerId)
	}
	if original.AddressPort != decoded.AddressPort {
		t.Errorf("AddressPort mismatch: got %v, want %v", decoded.AddressPort, original.AddressPort)
	}
	if original.Capacity != decoded.Capacity {
		t.Errorf("Capacity mismatch: got %v, want %v", decoded.Capacity, original.Capacity)
	}

	// 比较 Assignments 数组
	if len(original.Assignments) != len(decoded.Assignments) {
		t.Errorf("Assignments length mismatch: got %v, want %v", len(decoded.Assignments), len(original.Assignments))
	}
	for i, a := range original.Assignments {
		if decoded.Assignments[i].ShardId != a.ShardId {
			t.Errorf("Assignments[%d].ShardId mismatch: got %v, want %v", i, decoded.Assignments[i].ShardId, a.ShardId)
		}
		if decoded.Assignments[i].ReplicaIdx != a.ReplicaIdx {
			t.Errorf("Assignments[%d].ReplicaIdx mismatch: got %v, want %v", i, decoded.Assignments[i].ReplicaIdx, a.ReplicaIdx)
		}
		if decoded.Assignments[i].AsginmentId != a.AsginmentId {
			t.Errorf("Assignments[%d].AsginmentId mismatch: got %v, want %v", i, decoded.Assignments[i].AsginmentId, a.AsginmentId)
		}
	}
}
