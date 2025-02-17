package cougarjson

import (
	"testing"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
)

func TestPilotNodeJsonMarshal(t *testing.T) {
	// 使用 FakeTimeProvider 控制时间
	fakeTime := kcommon.NewFakeTimeProvider()
	fakeTime.WallTime = 1234567890000
	fakeTime.MonoTime = 1234567890000

	kcommon.RunWithTimeProvider(fakeTime, func() {
		tests := []struct {
			name     string
			input    *PilotNodeJson
			expected string
		}{
			{
				name: "完整字段",
				input: func() *PilotNodeJson {
					node := NewPilotNodeJson(
						"pilot-worker-75fffc88f9-fkbcm",
						"session-123",
						"initial setup",
					)
					node.Assignments = append(node.Assignments,
						NewPilotAssignmentJson("shard-1", 1, "asg-1", PAS_Active),
						NewPilotAssignmentJson("shard-2", 0, "asg-2", PAS_Completed),
					)
					return node
				}(),
				expected: `{"worker_id":"pilot-worker-75fffc88f9-fkbcm","session_id":"session-123","assignments":[{"shd":"shard-1","idx":1,"asg":"asg-1","sts":"active"},{"shd":"shard-2","asg":"asg-2","sts":"completed"}],"update_time_ms":1234567890000,"update_reason":"initial setup"}`,
			},
			{
				name: "最小字段",
				input: NewPilotNodeJson(
					"pilot-worker-75fffc88f9-xj9n2",
					"session-456",
					"minimal setup",
				),
				expected: `{"worker_id":"pilot-worker-75fffc88f9-xj9n2","session_id":"session-456","assignments":[],"update_time_ms":1234567890000,"update_reason":"minimal setup"}`,
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
	})
}

func TestPilotNodeJsonUnmarshal(t *testing.T) {
	// 使用 FakeTimeProvider 控制时间
	fakeTime := kcommon.NewFakeTimeProvider()
	fakeTime.WallTime = 1234567890000
	fakeTime.MonoTime = 1234567890000

	kcommon.RunWithTimeProvider(fakeTime, func() {
		tests := []struct {
			name      string
			input     string
			want      *PilotNodeJson
			wantPanic bool
			panicType string
		}{
			{
				name:  "完整字段",
				input: `{"worker_id":"pilot-worker-75fffc88f9-fkbcm","session_id":"session-123","assignments":[{"shd":"shard-1","idx":1,"asg":"asg-1","sts":"active"},{"shd":"shard-2","asg":"asg-2","sts":"completed"}],"update_time_ms":1234567890000,"update_reason":"initial setup"}`,
				want: func() *PilotNodeJson {
					node := NewPilotNodeJson(
						"pilot-worker-75fffc88f9-fkbcm",
						"session-123",
						"initial setup",
					)
					node.Assignments = append(node.Assignments,
						NewPilotAssignmentJson("shard-1", 1, "asg-1", PAS_Active),
						NewPilotAssignmentJson("shard-2", 0, "asg-2", PAS_Completed),
					)
					return node
				}(),
				wantPanic: false,
			},
			{
				name:  "最小字段",
				input: `{"worker_id":"pilot-worker-75fffc88f9-xj9n2","session_id":"session-456","assignments":[],"update_time_ms":1234567890000,"update_reason":"minimal setup"}`,
				want: func() *PilotNodeJson {
					node := NewPilotNodeJson(
						"pilot-worker-75fffc88f9-xj9n2",
						"session-456",
						"minimal setup",
					)
					return node
				}(),
				wantPanic: false,
			},
			{
				name:  "所有任务状态",
				input: `{"worker_id":"pilot-worker-75fffc88f9-r8t3v","session_id":"session-789","assignments":[{"shd":"shard-1","asg":"asg-1","sts":"active"},{"shd":"shard-2","asg":"asg-2","sts":"completed"}],"update_time_ms":1234567890000}`,
				want: func() *PilotNodeJson {
					node := NewPilotNodeJson(
						"pilot-worker-75fffc88f9-r8t3v",
						"session-789",
						"",
					)
					node.Assignments = append(node.Assignments,
						NewPilotAssignmentJson("shard-1", 0, "asg-1", PAS_Active),
						NewPilotAssignmentJson("shard-2", 0, "asg-2", PAS_Completed),
					)
					return node
				}(),
				wantPanic: false,
			},
			{
				name:      "格式错误",
				input:     `{"worker_id":123}`, // worker_id 应该是字符串
				wantPanic: true,
				panicType: "UnmarshalError",
			},
			{
				name:      "无效的任务状态",
				input:     `{"worker_id":"w1","session_id":"s1","assignments":[{"shd":"shard-1","asg":"asg-1","sts":"invalid"}]}`,
				wantPanic: true,
				panicType: "UnmarshalError",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				var got *PilotNodeJson
				var panicValue interface{}

				func() {
					defer func() {
						panicValue = recover()
					}()
					got = ParsePilotNodeJson(tt.input)
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
				if got.LastUpdateAtMs != tt.want.LastUpdateAtMs {
					t.Errorf("LastUpdateAtMs = %v, want %v", got.LastUpdateAtMs, tt.want.LastUpdateAtMs)
				}
				if got.LastUpdateReason != tt.want.LastUpdateReason {
					t.Errorf("LastUpdateReason = %v, want %v", got.LastUpdateReason, tt.want.LastUpdateReason)
				}

				// 验证任务列表
				if len(got.Assignments) != len(tt.want.Assignments) {
					t.Errorf("Assignments length = %v, want %v", len(got.Assignments), len(tt.want.Assignments))
				}
				for i, wantAssignment := range tt.want.Assignments {
					gotAssignment := got.Assignments[i]
					if gotAssignment.ShardId != wantAssignment.ShardId {
						t.Errorf("Assignments[%d].ShardId = %v, want %v", i, gotAssignment.ShardId, wantAssignment.ShardId)
					}
					if gotAssignment.ReplicaIdx != wantAssignment.ReplicaIdx {
						t.Errorf("Assignments[%d].ReplicaIdx = %v, want %v", i, gotAssignment.ReplicaIdx, wantAssignment.ReplicaIdx)
					}
					if gotAssignment.AsginmentId != wantAssignment.AsginmentId {
						t.Errorf("Assignments[%d].AsginmentId = %v, want %v", i, gotAssignment.AsginmentId, wantAssignment.AsginmentId)
					}
					if gotAssignment.State != wantAssignment.State {
						t.Errorf("Assignments[%d].State = %v, want %v", i, gotAssignment.State, wantAssignment.State)
					}
				}
			})
		}
	})
}

func TestNewPilotNodeJson(t *testing.T) {
	// 使用 FakeTimeProvider 控制时间
	fakeTime := kcommon.NewFakeTimeProvider()
	fakeTime.WallTime = 1234567890000
	fakeTime.MonoTime = 1234567890000

	kcommon.RunWithTimeProvider(fakeTime, func() {
		workerId := "pilot-worker-75fffc88f9-fkbcm"
		sessionId := "session-123"
		updateReason := "initial setup"

		node := NewPilotNodeJson(workerId, sessionId, updateReason)

		// 验证基本字段
		if node.WorkerId != workerId {
			t.Errorf("WorkerId = %v, want %v", node.WorkerId, workerId)
		}
		if node.SessionId != sessionId {
			t.Errorf("SessionId = %v, want %v", node.SessionId, sessionId)
		}
		if node.LastUpdateReason != updateReason {
			t.Errorf("LastUpdateReason = %v, want %v", node.LastUpdateReason, updateReason)
		}

		// 验证初始化的切片
		if node.Assignments == nil {
			t.Error("Assignments should be initialized to empty slice")
		}
		if len(node.Assignments) != 0 {
			t.Errorf("Assignments should be empty, got %v items", len(node.Assignments))
		}

		// 验证时间戳
		if node.LastUpdateAtMs != 1234567890000 {
			t.Errorf("LastUpdateAtMs = %v, want %v", node.LastUpdateAtMs, 1234567890000)
		}
	})
}

func TestNewPilotAssignmentJson(t *testing.T) {
	tests := []struct {
		name         string
		shardId      string
		replicaIdx   int
		assignmentId string
		state        PilotAssignmentState
	}{
		{
			name:         "Active Assignment",
			shardId:      "shard-1",
			replicaIdx:   1,
			assignmentId: "asg-1",
			state:        PAS_Active,
		},
		{
			name:         "Completed Assignment",
			shardId:      "shard-2",
			replicaIdx:   0,
			assignmentId: "asg-2",
			state:        PAS_Completed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assignment := NewPilotAssignmentJson(
				tt.shardId,
				tt.replicaIdx,
				tt.assignmentId,
				tt.state,
			)

			if assignment.ShardId != tt.shardId {
				t.Errorf("ShardId = %v, want %v", assignment.ShardId, tt.shardId)
			}
			if assignment.ReplicaIdx != tt.replicaIdx {
				t.Errorf("ReplicaIdx = %v, want %v", assignment.ReplicaIdx, tt.replicaIdx)
			}
			if assignment.AsginmentId != tt.assignmentId {
				t.Errorf("AsginmentId = %v, want %v", assignment.AsginmentId, tt.assignmentId)
			}
			if assignment.State != tt.state {
				t.Errorf("State = %v, want %v", assignment.State, tt.state)
			}
		})
	}
}

// TestPilotNodeJsonRoundTrip 测试序列化后再反序列化是否保持一致
func TestPilotNodeJsonRoundTrip(t *testing.T) {
	original := func() *PilotNodeJson {
		node := NewPilotNodeJson(
			"pilot-worker-75fffc88f9-fkbcm",
			"session-123",
			"round trip test",
		)
		node.Assignments = append(node.Assignments,
			NewPilotAssignmentJson("shard-1", 1, "asg-1", PAS_Active),
			NewPilotAssignmentJson("shard-2", 0, "asg-2", PAS_Completed),
		)
		return node
	}()

	// 序列化
	jsonStr := original.ToJson()

	// 反序列化
	decoded := ParsePilotNodeJson(jsonStr)

	// 比较结果
	if decoded.WorkerId != original.WorkerId {
		t.Errorf("WorkerId mismatch: got %v, want %v", decoded.WorkerId, original.WorkerId)
	}
	if decoded.SessionId != original.SessionId {
		t.Errorf("SessionId mismatch: got %v, want %v", decoded.SessionId, original.SessionId)
	}
	if decoded.LastUpdateReason != original.LastUpdateReason {
		t.Errorf("LastUpdateReason mismatch: got %v, want %v", decoded.LastUpdateReason, original.LastUpdateReason)
	}
	if decoded.LastUpdateAtMs != original.LastUpdateAtMs {
		t.Errorf("LastUpdateAtMs mismatch: got %v, want %v", decoded.LastUpdateAtMs, original.LastUpdateAtMs)
	}

	// 比较任务列表
	if len(decoded.Assignments) != len(original.Assignments) {
		t.Errorf("Assignments length mismatch: got %v, want %v", len(decoded.Assignments), len(original.Assignments))
	}
	for i, originalAssignment := range original.Assignments {
		decodedAssignment := decoded.Assignments[i]
		if decodedAssignment.ShardId != originalAssignment.ShardId {
			t.Errorf("Assignments[%d].ShardId mismatch: got %v, want %v", i, decodedAssignment.ShardId, originalAssignment.ShardId)
		}
		if decodedAssignment.ReplicaIdx != originalAssignment.ReplicaIdx {
			t.Errorf("Assignments[%d].ReplicaIdx mismatch: got %v, want %v", i, decodedAssignment.ReplicaIdx, originalAssignment.ReplicaIdx)
		}
		if decodedAssignment.AsginmentId != originalAssignment.AsginmentId {
			t.Errorf("Assignments[%d].AsginmentId mismatch: got %v, want %v", i, decodedAssignment.AsginmentId, originalAssignment.AsginmentId)
		}
		if decodedAssignment.State != originalAssignment.State {
			t.Errorf("Assignments[%d].State mismatch: got %v, want %v", i, decodedAssignment.State, originalAssignment.State)
		}
	}
}
