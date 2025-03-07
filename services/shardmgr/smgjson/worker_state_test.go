package smgjson

import (
	"testing"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

func TestWorkerStateJsonMarshal(t *testing.T) {
	tests := []struct {
		name     string
		input    *WorkerStateJson
		expected string
	}{
		{
			name: "空的工作节点状态",
			input: NewWorkerStateJson(
				data.WorkerId("worker-1"),
				data.SessionId("session-1"),
			),
			expected: `{"worker_id":"worker-1","session_id":"session-1","assignments":{}}`,
		},
		{
			name: "带有任务的工作节点状态",
			input: func() *WorkerStateJson {
				ws := NewWorkerStateJson(
					data.WorkerId("worker-2"),
					data.SessionId("session-2"),
				)
				ws.Assignments[data.AssignmentId("asg-1")] = NewAssignmentStateJson(data.ShardId("shard-1"), data.ReplicaIdx(0))
				ws.Assignments[data.AssignmentId("asg-2")] = NewAssignmentStateJson(data.ShardId("shard-2"), data.ReplicaIdx(0))
				return ws
			}(),
			expected: `{"worker_id":"worker-2","session_id":"session-2","assignments":{"asg-1":{"sid":"shard-1"},"asg-2":{"sid":"shard-2"}}}`,
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

func TestWorkerStateJsonUnmarshal(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      *WorkerStateJson
		wantPanic bool
		panicType string
	}{
		{
			name:  "空的工作节点状态",
			input: `{"worker_id":"worker-1","session_id":"session-1","assignments":{}}`,
			want: NewWorkerStateJson(
				data.WorkerId("worker-1"),
				data.SessionId("session-1"),
			),
			wantPanic: false,
		},
		{
			name:  "带有任务的工作节点状态",
			input: `{"worker_id":"worker-2","session_id":"session-2","assignments":{"asg-1":{},"asg-2":{}}}`,
			want: func() *WorkerStateJson {
				ws := NewWorkerStateJson(
					data.WorkerId("worker-2"),
					data.SessionId("session-2"),
				)
				ws.Assignments[data.AssignmentId("asg-1")] = NewAssignmentStateJson(data.ShardId("shard-1"), data.ReplicaIdx(0))
				ws.Assignments[data.AssignmentId("asg-2")] = NewAssignmentStateJson(data.ShardId("shard-2"), data.ReplicaIdx(0))
				return ws
			}(),
			wantPanic: false,
		},
		{
			name:      "无效的 JSON",
			input:     `{invalid_json}`,
			wantPanic: true,
			panicType: "UnmarshalError",
		},
		{
			name:      "缺少必需字段",
			input:     `{"worker_id":"worker-1"}`,
			wantPanic: true,
			panicType: "UnmarshalError",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got *WorkerStateJson
			var panicValue interface{}

			func() {
				defer func() {
					panicValue = recover()
				}()
				got = WorkerStateJsonFromJson(tt.input)
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

			// 验证基本字段
			if got.WorkerId != tt.want.WorkerId {
				t.Errorf("WorkerId = %v, want %v", got.WorkerId, tt.want.WorkerId)
			}
			if got.SessionId != tt.want.SessionId {
				t.Errorf("SessionId = %v, want %v", got.SessionId, tt.want.SessionId)
			}

			// 验证任务列表
			if len(got.Assignments) != len(tt.want.Assignments) {
				t.Errorf("Assignments length = %v, want %v", len(got.Assignments), len(tt.want.Assignments))
			}
			for id := range tt.want.Assignments {
				if _, ok := got.Assignments[id]; !ok {
					t.Errorf("Assignment %v not found", id)
				}
			}
		})
	}
}

// TestWorkerStateJsonRoundTrip 测试序列化后再反序列化是否保持一致
func TestWorkerStateJsonRoundTrip(t *testing.T) {
	original := func() *WorkerStateJson {
		ws := NewWorkerStateJson(
			data.WorkerId("worker-1"),
			data.SessionId("session-1"),
		)
		ws.Assignments[data.AssignmentId("asg-1")] = NewAssignmentStateJson(data.ShardId("shard-1"), data.ReplicaIdx(0))
		ws.Assignments[data.AssignmentId("asg-2")] = NewAssignmentStateJson(data.ShardId("shard-2"), data.ReplicaIdx(0))
		return ws
	}()

	// 序列化
	jsonStr := original.ToJson()

	// 反序列化
	decoded := WorkerStateJsonFromJson(jsonStr)

	// 比较结果
	if decoded.WorkerId != original.WorkerId {
		t.Errorf("WorkerId mismatch: got %v, want %v", decoded.WorkerId, original.WorkerId)
	}
	if decoded.SessionId != original.SessionId {
		t.Errorf("SessionId mismatch: got %v, want %v", decoded.SessionId, original.SessionId)
	}

	// 比较任务列表
	if len(decoded.Assignments) != len(original.Assignments) {
		t.Errorf("Assignments length mismatch: got %v, want %v", len(decoded.Assignments), len(original.Assignments))
	}
	for id := range original.Assignments {
		if _, ok := decoded.Assignments[id]; !ok {
			t.Errorf("Assignment %v not found after round trip", id)
		}
	}
}

// TestNewWorkerStateJson 测试创建新的 WorkerStateJson 实例
func TestNewWorkerStateJson(t *testing.T) {
	workerId := data.WorkerId("worker-1")
	sessionId := data.SessionId("session-1")

	ws := NewWorkerStateJson(workerId, sessionId)

	// 验证基本字段
	if ws.WorkerId != workerId {
		t.Errorf("WorkerId = %v, want %v", ws.WorkerId, workerId)
	}
	if ws.SessionId != sessionId {
		t.Errorf("SessionId = %v, want %v", ws.SessionId, sessionId)
	}

	// 验证初始化的映射
	if ws.Assignments == nil {
		t.Error("Assignments should be initialized to empty map")
	}
	if len(ws.Assignments) != 0 {
		t.Errorf("Assignments should be empty, got %v items", len(ws.Assignments))
	}
}
