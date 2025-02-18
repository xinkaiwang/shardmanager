package smgjson

import (
	"testing"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

func TestShardStateJsonMarshal(t *testing.T) {
	tests := []struct {
		name     string
		input    *ShardStateJson
		expected string
	}{
		{
			name: "空的分片状态",
			input: NewShardStateJson(
				data.ShardId("shard-1"),
			),
			expected: `{"shard_name":"shard-1","resplicas":{}}`,
		},
		{
			name: "带有副本的分片状态",
			input: func() *ShardStateJson {
				ss := NewShardStateJson(
					data.ShardId("shard-2"),
				)
				ss.Resplicas[0] = &ReplicaStateJson{}
				ss.Resplicas[1] = &ReplicaStateJson{}
				return ss
			}(),
			expected: `{"shard_name":"shard-2","resplicas":{"0":{},"1":{}}}`,
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

func TestShardStateJsonUnmarshal(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      *ShardStateJson
		wantPanic bool
		panicType string
	}{
		{
			name:  "空的分片状态",
			input: `{"shard_name":"shard-1","resplicas":{}}`,
			want: NewShardStateJson(
				data.ShardId("shard-1"),
			),
			wantPanic: false,
		},
		{
			name:  "带有副本的分片状态",
			input: `{"shard_name":"shard-2","resplicas":{"0":{},"1":{}}}`,
			want: func() *ShardStateJson {
				ss := NewShardStateJson(
					data.ShardId("shard-2"),
				)
				ss.Resplicas[0] = &ReplicaStateJson{}
				ss.Resplicas[1] = &ReplicaStateJson{}
				return ss
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
			input:     `{"resplicas":{}}`,
			wantPanic: true,
			panicType: "UnmarshalError",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got *ShardStateJson
			var panicValue interface{}

			func() {
				defer func() {
					panicValue = recover()
				}()
				got = ShardStateJsonFromJson(tt.input)
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
			if got.ShardName != tt.want.ShardName {
				t.Errorf("ShardName = %v, want %v", got.ShardName, tt.want.ShardName)
			}

			// 验证副本列表
			if len(got.Resplicas) != len(tt.want.Resplicas) {
				t.Errorf("Resplicas length = %v, want %v", len(got.Resplicas), len(tt.want.Resplicas))
			}
			for idx := range tt.want.Resplicas {
				if _, ok := got.Resplicas[idx]; !ok {
					t.Errorf("Replica %v not found", idx)
				}
			}
		})
	}
}

// TestShardStateJsonRoundTrip 测试序列化后再反序列化是否保持一致
func TestShardStateJsonRoundTrip(t *testing.T) {
	original := func() *ShardStateJson {
		ss := NewShardStateJson(
			data.ShardId("shard-1"),
		)
		ss.Resplicas[0] = &ReplicaStateJson{}
		ss.Resplicas[1] = &ReplicaStateJson{}
		return ss
	}()

	// 序列化
	jsonStr := original.ToJson()

	// 反序列化
	decoded := ShardStateJsonFromJson(jsonStr)

	// 比较结果
	if decoded.ShardName != original.ShardName {
		t.Errorf("ShardName mismatch: got %v, want %v", decoded.ShardName, original.ShardName)
	}

	// 比较副本列表
	if len(decoded.Resplicas) != len(original.Resplicas) {
		t.Errorf("Resplicas length mismatch: got %v, want %v", len(decoded.Resplicas), len(original.Resplicas))
	}
	for idx := range original.Resplicas {
		if _, ok := decoded.Resplicas[idx]; !ok {
			t.Errorf("Replica %v not found after round trip", idx)
		}
	}
}

// TestNewShardStateJson 测试创建新的 ShardStateJson 实例
func TestNewShardStateJson(t *testing.T) {
	shardName := data.ShardId("shard-1")

	ss := NewShardStateJson(shardName)

	// 验证基本字段
	if ss.ShardName != shardName {
		t.Errorf("ShardName = %v, want %v", ss.ShardName, shardName)
	}

	// 验证初始化的映射
	if ss.Resplicas == nil {
		t.Error("Resplicas should be initialized to empty map")
	}
	if len(ss.Resplicas) != 0 {
		t.Errorf("Resplicas should be empty, got %v items", len(ss.Resplicas))
	}
}
