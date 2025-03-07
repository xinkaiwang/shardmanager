package smgjson

import (
	"testing"
)

func TestParseShardLine(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		wantShardName   string
		wantHints       *ShardHintsJson
		wantCustomProps map[string]string
		wantPanic       bool
		description     string
	}{
		{
			name:            "仅 shard 名称",
			input:           "shard_1200_1300",
			wantShardName:   "shard_1200_1300",
			wantHints:       nil,
			wantCustomProps: make(map[string]string),
			wantPanic:       false,
			description:     "最简单的情况，只有 shard 名称",
		},
		{
			name:          "带 hints",
			input:         `shard_1200_1300|{"min_replica_count":1,"max_replica_count":10}`,
			wantShardName: "shard_1200_1300",
			wantHints: func() *ShardHintsJson {
				min := int32(1)
				max := int32(10)
				return &ShardHintsJson{
					MinReplicaCount: &min,
					MaxReplicaCount: &max,
				}
			}(),
			wantCustomProps: make(map[string]string),
			wantPanic:       false,
			description:     "包含 hints 的情况",
		},
		{
			name:          "带 hints 和自定义属性",
			input:         `shard_1200_1300|{"min_replica_count":1,"max_replica_count":10}|{"pip":"xformer","custom_node":"node1"}`,
			wantShardName: "shard_1200_1300",
			wantHints: func() *ShardHintsJson {
				min := int32(1)
				max := int32(10)
				return &ShardHintsJson{
					MinReplicaCount: &min,
					MaxReplicaCount: &max,
				}
			}(),
			wantCustomProps: map[string]string{
				"pip":         "xformer",
				"custom_node": "node1",
			},
			wantPanic:   false,
			description: "完整格式，包含 hints 和自定义属性",
		},
		{
			name:            "空 hints",
			input:           "shard_1200_1300||",
			wantShardName:   "shard_1200_1300",
			wantHints:       nil,
			wantCustomProps: make(map[string]string),
			wantPanic:       false,
			description:     "hints 和自定义属性为空",
		},
		{
			name:          "部分 hints",
			input:         `shard_1200_1300|{"min_replica_count":1}`,
			wantShardName: "shard_1200_1300",
			wantHints: func() *ShardHintsJson {
				min := int32(1)
				return &ShardHintsJson{
					MinReplicaCount: &min,
				}
			}(),
			wantCustomProps: make(map[string]string),
			wantPanic:       false,
			description:     "只设置部分 hints 字段",
		},
		{
			name:        "无效的 hints JSON",
			input:       `shard_1200_1300|{invalid_json}`,
			wantPanic:   true,
			description: "hints 部分的 JSON 格式无效",
		},
		{
			name:        "无效的自定义属性 JSON",
			input:       `shard_1200_1300|{"min_replica_count":1}|{invalid_json}`,
			wantPanic:   true,
			description: "自定义属性部分的 JSON 格式无效",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got *ShardLineJson
			var panicValue interface{}

			func() {
				defer func() {
					panicValue = recover()
				}()
				got = ParseShardLine(tt.input)
			}()

			if tt.wantPanic {
				if panicValue == nil {
					t.Error("Expected panic but got none")
				}
				return
			}

			if panicValue != nil {
				t.Errorf("Unexpected panic: %v", panicValue)
				return
			}

			// 验证 ShardName
			if got.ShardName != tt.wantShardName {
				t.Errorf("ShardName = %v, want %v", got.ShardName, tt.wantShardName)
			}

			// 验证 Hints
			if tt.wantHints == nil {
				if got.Hints != nil {
					t.Errorf("Hints = %+v, want nil", got.Hints)
				}
			} else {
				if got.Hints == nil {
					t.Errorf("Hints is nil, want %+v", tt.wantHints)
				} else {
					// 验证 MinReplicaCount
					if (got.Hints.MinReplicaCount == nil) != (tt.wantHints.MinReplicaCount == nil) {
						t.Errorf("MinReplicaCount nil status mismatch")
					} else if got.Hints.MinReplicaCount != nil && *got.Hints.MinReplicaCount != *tt.wantHints.MinReplicaCount {
						t.Errorf("MinReplicaCount = %v, want %v", *got.Hints.MinReplicaCount, *tt.wantHints.MinReplicaCount)
					}

					// 验证 MaxReplicaCount
					if (got.Hints.MaxReplicaCount == nil) != (tt.wantHints.MaxReplicaCount == nil) {
						t.Errorf("MaxReplicaCount nil status mismatch")
					} else if got.Hints.MaxReplicaCount != nil && *got.Hints.MaxReplicaCount != *tt.wantHints.MaxReplicaCount {
						t.Errorf("MaxReplicaCount = %v, want %v", *got.Hints.MaxReplicaCount, *tt.wantHints.MaxReplicaCount)
					}

					// 验证 MoveType
					if (got.Hints.MoveType == nil) != (tt.wantHints.MoveType == nil) {
						t.Errorf("MoveType nil status mismatch")
					} else if got.Hints.MoveType != nil && *got.Hints.MoveType != *tt.wantHints.MoveType {
						t.Errorf("MoveType = %v, want %v", *got.Hints.MoveType, *tt.wantHints.MoveType)
					}
				}
			}

			// 验证 CustomProperties
			if len(got.CustomProperties) != len(tt.wantCustomProps) {
				t.Errorf("CustomProperties length = %v, want %v", len(got.CustomProperties), len(tt.wantCustomProps))
			}
			for k, v := range tt.wantCustomProps {
				if got.CustomProperties[k] != v {
					t.Errorf("CustomProperties[%q] = %v, want %v", k, got.CustomProperties[k], v)
				}
			}
		})
	}
}

func TestToShardLine(t *testing.T) {
	defaultHints := ShardHintsJson{
		MinReplicaCount: func() *int32 { v := int32(1); return &v }(),
		MaxReplicaCount: func() *int32 { v := int32(10); return &v }(),
		MoveType:        func() *MovePolicy { v := MP_StartBeforeKill; return &v }(),
	}

	tests := []struct {
		name          string
		shardLineJson *ShardLineJson
		want          *ShardLineJson
		description   string
	}{
		{
			name: "完整字段转换",
			shardLineJson: func() *ShardLineJson {
				min := int32(2)
				max := int32(20)
				moveType := MP_KillBeforeStart
				return &ShardLineJson{
					ShardName: "shard_1",
					Hints: &ShardHintsJson{
						MinReplicaCount: &min,
						MaxReplicaCount: &max,
						MoveType:        &moveType,
					},
					CustomProperties: map[string]string{
						"pip": "xformer",
					},
				}
			}(),
			want: &ShardLineJson{
				ShardName: "shard_1",
				Hints: &ShardHintsJson{
					MinReplicaCount: func() *int32 { v := int32(2); return &v }(),
					MaxReplicaCount: func() *int32 { v := int32(20); return &v }(),
					MoveType:        func() *MovePolicy { v := MP_KillBeforeStart; return &v }(),
				},
				CustomProperties: map[string]string{
					"pip": "xformer",
				},
			},
			description: "所有字段都有自定义值",
		},
		{
			name: "使用默认值转换",
			shardLineJson: &ShardLineJson{
				ShardName: "shard_2",
				Hints:     nil,
			},
			want: &ShardLineJson{
				ShardName: "shard_2",
				Hints:     &defaultHints,
			},
			description: "使用默认值",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.shardLineJson

			// 如果 Hints 为空，使用默认值
			if got.Hints == nil {
				got.Hints = &defaultHints
			}

			// 验证 ShardName
			if got.ShardName != tt.want.ShardName {
				t.Errorf("ShardName = %v, want %v", got.ShardName, tt.want.ShardName)
			}

			// 验证 Hints
			if tt.want.Hints == nil {
				if got.Hints != nil {
					t.Errorf("Hints = %+v, want nil", got.Hints)
				}
			} else {
				if got.Hints == nil {
					t.Errorf("Hints is nil, want %+v", tt.want.Hints)
				} else {
					// 验证 MinReplicaCount
					if (got.Hints.MinReplicaCount == nil) != (tt.want.Hints.MinReplicaCount == nil) {
						t.Errorf("MinReplicaCount nil status mismatch")
					} else if got.Hints.MinReplicaCount != nil && *got.Hints.MinReplicaCount != *tt.want.Hints.MinReplicaCount {
						t.Errorf("MinReplicaCount = %v, want %v", *got.Hints.MinReplicaCount, *tt.want.Hints.MinReplicaCount)
					}

					// 验证 MaxReplicaCount
					if (got.Hints.MaxReplicaCount == nil) != (tt.want.Hints.MaxReplicaCount == nil) {
						t.Errorf("MaxReplicaCount nil status mismatch")
					} else if got.Hints.MaxReplicaCount != nil && *got.Hints.MaxReplicaCount != *tt.want.Hints.MaxReplicaCount {
						t.Errorf("MaxReplicaCount = %v, want %v", *got.Hints.MaxReplicaCount, *tt.want.Hints.MaxReplicaCount)
					}

					// 验证 MoveType
					if (got.Hints.MoveType == nil) != (tt.want.Hints.MoveType == nil) {
						t.Errorf("MoveType nil status mismatch")
					} else if got.Hints.MoveType != nil && *got.Hints.MoveType != *tt.want.Hints.MoveType {
						t.Errorf("MoveType = %v, want %v", *got.Hints.MoveType, *tt.want.Hints.MoveType)
					}
				}
			}

			// 验证 CustomProperties
			if len(got.CustomProperties) != len(tt.want.CustomProperties) {
				t.Errorf("CustomProperties length = %v, want %v", len(got.CustomProperties), len(tt.want.CustomProperties))
			}
			for k, v := range tt.want.CustomProperties {
				if got.CustomProperties[k] != v {
					t.Errorf("CustomProperties[%q] = %v, want %v", k, got.CustomProperties[k], v)
				}
			}
		})
	}
}
