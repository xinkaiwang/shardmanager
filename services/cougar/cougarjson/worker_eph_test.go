package cougarjson

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
)

func TestWorkerEphJsonMarshal(t *testing.T) {
	now := int64(1234567890000)

	tests := []struct {
		name     string
		input    *WorkerEphJson
		expected string
		wantErr  bool
	}{
		{
			name: "完整字段",
			input: func() *WorkerEphJson {
				worker := NewWorkerEphJson(
					"cougar-worker-75fffc88f9-fkbcm",
					"session-123",
					now,
					10,
				)
				worker.AddressPort = "10.0.0.32:8080"
				worker.Properties["region"] = "us-west"
				worker.Properties["type"] = "compute"
				worker.Assignments = append(worker.Assignments,
					NewAssignmentJson("shard-1", 1, "asg-1", CAS_Active),
				)
				worker.LastUpdateAtMs = now + 1000
				worker.LastUpdateReason = "periodic update"
				return worker
			}(),
			expected: `{"worker_id":"cougar-worker-75fffc88f9-fkbcm","addr_port":"10.0.0.32:8080","sesssion_id":"session-123","start_time_ms":1234567890000,"capacity":10,"properties":{"region":"us-west","type":"compute"},"assignments":[{"shd":"shard-1","idx":1,"asg":"asg-1","sts":"active"}],"update_time_ms":1234567891000,"update_reason":"periodic update"}`,
			wantErr:  false,
		},
		{
			name: "最小字段",
			input: NewWorkerEphJson(
				"cougar-worker-75fffc88f9-xj9n2",
				"session-456",
				now,
				20,
			),
			expected: `{"worker_id":"cougar-worker-75fffc88f9-xj9n2","sesssion_id":"session-456","start_time_ms":1234567890000,"capacity":20}`,
			wantErr:  false,
		},
		{
			name: "空可选字段",
			input: NewWorkerEphJson(
				"cougar-worker-75fffc88f9-p4m7k",
				"session-789",
				now,
				30,
			),
			expected: `{"worker_id":"cougar-worker-75fffc88f9-p4m7k","sesssion_id":"session-789","start_time_ms":1234567890000,"capacity":30}`,
			wantErr:  false,
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

func TestWorkerEphJsonUnmarshal(t *testing.T) {
	now := int64(1234567890000)

	tests := []struct {
		name      string
		input     string
		want      *WorkerEphJson
		wantPanic bool
		panicType string
	}{
		{
			name:  "完整字段",
			input: `{"worker_id":"cougar-worker-75fffc88f9-fkbcm","addr_port":"10.0.0.32:8080","sesssion_id":"session-123","start_time_ms":1234567890000,"capacity":10,"properties":{"region":"us-west","type":"compute"},"assignments":[{"shd":"shard-1","idx":1,"asg":"asg-1","sts":"active"}],"update_time_ms":1234567891000,"update_reason":"periodic update"}`,
			want: func() *WorkerEphJson {
				worker := NewWorkerEphJson(
					"cougar-worker-75fffc88f9-fkbcm",
					"session-123",
					now,
					10,
				)
				worker.AddressPort = "10.0.0.32:8080"
				worker.Properties["region"] = "us-west"
				worker.Properties["type"] = "compute"
				worker.Assignments = append(worker.Assignments,
					NewAssignmentJson("shard-1", 1, "asg-1", CAS_Active),
				)
				worker.LastUpdateAtMs = now + 1000
				worker.LastUpdateReason = "periodic update"
				return worker
			}(),
			wantPanic: false,
		},
		{
			name:  "最小字段",
			input: `{"worker_id":"cougar-worker-75fffc88f9-xj9n2","sesssion_id":"session-456","start_time_ms":1234567890000,"capacity":20}`,
			want: NewWorkerEphJson(
				"cougar-worker-75fffc88f9-xj9n2",
				"session-456",
				now,
				20,
			),
			wantPanic: false,
		},
		{
			name:  "空可选字段",
			input: `{"worker_id":"cougar-worker-75fffc88f9-p4m7k","sesssion_id":"session-789","start_time_ms":1234567890000,"capacity":30}`,
			want: NewWorkerEphJson(
				"cougar-worker-75fffc88f9-p4m7k",
				"session-789",
				now,
				30,
			),
			wantPanic: false,
		},
		{
			name:  "缺少必需字段",
			input: `{"worker_id":"cougar-worker-75fffc88f9-r8t3v","capacity":40}`,
			want: NewWorkerEphJson(
				"cougar-worker-75fffc88f9-r8t3v",
				"",
				0,
				40,
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
			var got *WorkerEphJson
			var panicValue interface{}

			func() {
				defer func() {
					panicValue = recover()
				}()
				got = WorkerEphJsonFromJson(tt.input)
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
			if got.SessionId != tt.want.SessionId {
				t.Errorf("SessionId = %v, want %v", got.SessionId, tt.want.SessionId)
			}
			if got.StartTimeMs != tt.want.StartTimeMs {
				t.Errorf("StartTimeMs = %v, want %v", got.StartTimeMs, tt.want.StartTimeMs)
			}
			if got.Capacity != tt.want.Capacity {
				t.Errorf("Capacity = %v, want %v", got.Capacity, tt.want.Capacity)
			}
			if got.LastUpdateAtMs != tt.want.LastUpdateAtMs {
				t.Errorf("LastUpdateAtMs = %v, want %v", got.LastUpdateAtMs, tt.want.LastUpdateAtMs)
			}
			if got.LastUpdateReason != tt.want.LastUpdateReason {
				t.Errorf("LastUpdateReason = %v, want %v", got.LastUpdateReason, tt.want.LastUpdateReason)
			}
			// 比较 Properties 映射
			if len(got.Properties) != len(tt.want.Properties) {
				t.Errorf("Properties length = %v, want %v", len(got.Properties), len(tt.want.Properties))
			}
			for k, v := range tt.want.Properties {
				if got.Properties[k] != v {
					t.Errorf("Properties[%q] = %v, want %v", k, got.Properties[k], v)
				}
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
				if got.Assignments[i].State != a.State {
					t.Errorf("Assignments[%d].State = %v, want %v", i, got.Assignments[i].State, a.State)
				}
			}
		})
	}
}

// TestWorkerEphJsonRoundTrip 测试序列化后再反序列化是否保持一致
func TestWorkerEphJsonRoundTrip(t *testing.T) {
	now := int64(1234567890000)

	original := func() *WorkerEphJson {
		worker := NewWorkerEphJson(
			"cougar-worker-75fffc88f9-fkbcm",
			"session-123",
			now,
			10,
		)
		worker.AddressPort = "10.0.0.32:8080"
		worker.Properties["region"] = "us-west"
		worker.Properties["type"] = "compute"
		worker.Assignments = append(worker.Assignments,
			NewAssignmentJson("shard-1", 1, "asg-1", CAS_Active),
		)
		worker.LastUpdateAtMs = now + 1000
		worker.LastUpdateReason = "periodic update"
		return worker
	}()

	// 序列化
	jsonStr := original.ToJson()

	// 反序列化
	decoded := WorkerEphJsonFromJson(jsonStr)

	// 比较结果
	if original.WorkerId != decoded.WorkerId {
		t.Errorf("WorkerId mismatch: got %v, want %v", decoded.WorkerId, original.WorkerId)
	}
	if original.AddressPort != decoded.AddressPort {
		t.Errorf("AddressPort mismatch: got %v, want %v", decoded.AddressPort, original.AddressPort)
	}
	if original.SessionId != decoded.SessionId {
		t.Errorf("SessionId mismatch: got %v, want %v", decoded.SessionId, original.SessionId)
	}
	if original.StartTimeMs != decoded.StartTimeMs {
		t.Errorf("StartTimeMs mismatch: got %v, want %v", decoded.StartTimeMs, original.StartTimeMs)
	}
	if original.Capacity != decoded.Capacity {
		t.Errorf("Capacity mismatch: got %v, want %v", decoded.Capacity, original.Capacity)
	}
	if original.LastUpdateAtMs != decoded.LastUpdateAtMs {
		t.Errorf("LastUpdateAtMs mismatch: got %v, want %v", decoded.LastUpdateAtMs, original.LastUpdateAtMs)
	}
	if original.LastUpdateReason != decoded.LastUpdateReason {
		t.Errorf("LastUpdateReason mismatch: got %v, want %v", decoded.LastUpdateReason, original.LastUpdateReason)
	}
	// 比较 Properties 映射
	if len(original.Properties) != len(decoded.Properties) {
		t.Errorf("Properties length mismatch: got %v, want %v", len(decoded.Properties), len(original.Properties))
	}
	for k, v := range original.Properties {
		if decoded.Properties[k] != v {
			t.Errorf("Properties[%q] mismatch: got %v, want %v", k, decoded.Properties[k], v)
		}
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
		if decoded.Assignments[i].State != a.State {
			t.Errorf("Assignments[%d].State mismatch: got %v, want %v", i, decoded.Assignments[i].State, a.State)
		}
	}
}

func TestNewWorkerEphJson(t *testing.T) {
	now := int64(1234567890000)
	worker := NewWorkerEphJson(
		"cougar-worker-75fffc88f9-fkbcm",
		"session-123",
		now,
		10,
	)

	// 验证必需字段
	if worker.WorkerId != "cougar-worker-75fffc88f9-fkbcm" {
		t.Errorf("WorkerId = %v, want %v", worker.WorkerId, "cougar-worker-75fffc88f9-fkbcm")
	}
	if worker.SessionId != "session-123" {
		t.Errorf("SessionId = %v, want %v", worker.SessionId, "session-123")
	}
	if worker.StartTimeMs != now {
		t.Errorf("StartTimeMs = %v, want %v", worker.StartTimeMs, now)
	}
	if worker.Capacity != 10 {
		t.Errorf("Capacity = %v, want %v", worker.Capacity, 10)
	}

	// 验证初始化的映射和数组
	if worker.Properties == nil {
		t.Error("Properties should be initialized to empty map")
	}
	if len(worker.Properties) != 0 {
		t.Errorf("Properties should be empty, got %v items", len(worker.Properties))
	}
	if worker.Assignments == nil {
		t.Error("Assignments should be initialized to empty slice")
	}
	if len(worker.Assignments) != 0 {
		t.Errorf("Assignments should be empty, got %v items", len(worker.Assignments))
	}

	// 验证可选字段为零值
	if worker.AddressPort != "" {
		t.Errorf("AddressPort should be empty, got %v", worker.AddressPort)
	}
	if worker.LastUpdateAtMs != 0 {
		t.Errorf("LastUpdateAtMs should be 0, got %v", worker.LastUpdateAtMs)
	}
	if worker.LastUpdateReason != "" {
		t.Errorf("LastUpdateReason should be empty, got %v", worker.LastUpdateReason)
	}
}

func TestNewAssignmentJson(t *testing.T) {
	tests := []struct {
		name         string
		shardId      string
		replicaIdx   int
		assignmentId string
		state        CougarAssignmentState
	}{
		{
			name:         "Active Assignment",
			shardId:      "shard-1",
			replicaIdx:   1,
			assignmentId: "asg-1",
			state:        CAS_Active,
		},
		{
			name:         "Pending Assignment",
			shardId:      "shard-2",
			replicaIdx:   0,
			assignmentId: "asg-2",
			state:        CAS_Pending,
		},
		{
			name:         "Failed Assignment",
			shardId:      "shard-3",
			replicaIdx:   2,
			assignmentId: "asg-3",
			state:        CAS_Failed,
		},
		{
			name:         "Completed Assignment",
			shardId:      "shard-4",
			replicaIdx:   3,
			assignmentId: "asg-4",
			state:        CAS_Completed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assignment := NewAssignmentJson(
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

			// 测试 JSON 序列化
			data, err := json.Marshal(assignment)
			if err != nil {
				t.Fatalf("json.Marshal() failed: %v", err)
			}

			// 验证 JSON 格式
			var expected string
			if tt.replicaIdx == 0 {
				// ReplicaIdx 为 0 时，由于 omitempty，不会出现在 JSON 中
				expected = fmt.Sprintf(`{"shd":"%s","asg":"%s","sts":"%s"}`,
					tt.shardId, tt.assignmentId, tt.state)
			} else {
				expected = fmt.Sprintf(`{"shd":"%s","idx":%d,"asg":"%s","sts":"%s"}`,
					tt.shardId, tt.replicaIdx, tt.assignmentId, tt.state)
			}
			if string(data) != expected {
				t.Errorf("json.Marshal() = %v, want %v", string(data), expected)
			}
		})
	}
}
