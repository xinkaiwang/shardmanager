package smgjson

import (
	"testing"
)

func TestParseShardPlan(t *testing.T) {
	defaultHints := ShardHints{
		MinReplicaCount: 1,
		MaxReplicaCount: 10,
		MoveType:        MP_StartBeforeKill,
	}

	tests := []struct {
		name        string
		input       string
		want        *ShardPlan
		description string
	}{
		{
			name: "多行 shard plan",
			input: `shard_1
shard_2|{"min_replica_count":2,"max_replica_count":20}
shard_3|{"min_replica_count":3}|{"pip":"xformer"}`,
			want: &ShardPlan{
				ShardLines: []*ShardLine{
					{
						ShardName: "shard_1",
						Hints:     defaultHints,
					},
					{
						ShardName: "shard_2",
						Hints: ShardHints{
							MinReplicaCount: 2,
							MaxReplicaCount: 20,
							MoveType:        MP_StartBeforeKill,
						},
					},
					{
						ShardName: "shard_3",
						Hints: ShardHints{
							MinReplicaCount: 3,
							MaxReplicaCount: 10,
							MoveType:        MP_StartBeforeKill,
						},
						CustomProperties: map[string]string{
							"pip": "xformer",
						},
					},
				},
			},
			description: "包含多行不同格式的 shard line",
		},
		{
			name: "空行处理",
			input: `shard_1

shard_2

`,
			want: &ShardPlan{
				ShardLines: []*ShardLine{
					{
						ShardName: "shard_1",
						Hints:     defaultHints,
					},
					{
						ShardName: "shard_2",
						Hints:     defaultHints,
					},
				},
			},
			description: "包含空行，应该被正确处理",
		},
		{
			name:  "空输入",
			input: "",
			want: &ShardPlan{
				ShardLines: []*ShardLine{},
			},
			description: "空输入应该返回空的 ShardPlan",
		},
		{
			name: "注释和空白处理",
			input: `# 这是注释
shard_1  # 行尾注释
  shard_2  # 前导和尾随空格
	shard_3	# 制表符`,
			want: &ShardPlan{
				ShardLines: []*ShardLine{
					{
						ShardName: "shard_1",
						Hints:     defaultHints,
					},
					{
						ShardName: "shard_2",
						Hints:     defaultHints,
					},
					{
						ShardName: "shard_3",
						Hints:     defaultHints,
					},
				},
			},
			description: "正确处理注释和各种空白字符",
		},
		{
			name: "混合格式和注释",
			input: `# 配置文件开始
shard_1|{"min_replica_count":2}  # 基本配置
# 下面是高级配置
shard_2|{"max_replica_count":20,"move_type":"kill_before_start"}|{"region":"us-west"}  # 带自定义属性
  # 中间的注释
shard_3  # 使用默认值`,
			want: &ShardPlan{
				ShardLines: []*ShardLine{
					{
						ShardName: "shard_1",
						Hints: ShardHints{
							MinReplicaCount: 2,
							MaxReplicaCount: 10,
							MoveType:        MP_StartBeforeKill,
						},
					},
					{
						ShardName: "shard_2",
						Hints: ShardHints{
							MinReplicaCount: 1,
							MaxReplicaCount: 20,
							MoveType:        MP_KillBeforeStart,
						},
						CustomProperties: map[string]string{
							"region": "us-west",
						},
					},
					{
						ShardName: "shard_3",
						Hints:     defaultHints,
					},
				},
			},
			description: "混合使用不同格式和注释",
		},
		{
			name: "特殊格式处理",
			input: `# Windows 风格的换行（CRLF）
shard_1
# 多个连续空行和注释


# 下一个配置
shard_2|{"min_replica_count":5}|{"env":"prod"}  # 带注释`,
			want: &ShardPlan{
				ShardLines: []*ShardLine{
					{
						ShardName: "shard_1",
						Hints:     defaultHints,
					},
					{
						ShardName: "shard_2",
						Hints: ShardHints{
							MinReplicaCount: 5,
							MaxReplicaCount: 10,
							MoveType:        MP_StartBeforeKill,
						},
						CustomProperties: map[string]string{
							"env": "prod",
						},
					},
				},
			},
			description: "处理特殊格式和连续空行",
		},
		{
			name: "全注释和空白",
			input: `# 这是一个注释
   # 这是缩进的注释
	# 这是制表符注释

# 最后一行注释`,
			want: &ShardPlan{
				ShardLines: []*ShardLine{},
			},
			description: "文件只包含注释和空白字符",
		},
		{
			name: "复杂的移动策略配置",
			input: `shard_1|{"move_type":"start_before_kill"}
shard_2|{"move_type":"kill_before_start"}
shard_3|{"move_type":"concurrent"}
shard_4  # 默认移动策略`,
			want: &ShardPlan{
				ShardLines: []*ShardLine{
					{
						ShardName: "shard_1",
						Hints: ShardHints{
							MinReplicaCount: 1,
							MaxReplicaCount: 10,
							MoveType:        MP_StartBeforeKill,
						},
					},
					{
						ShardName: "shard_2",
						Hints: ShardHints{
							MinReplicaCount: 1,
							MaxReplicaCount: 10,
							MoveType:        MP_KillBeforeStart,
						},
					},
					{
						ShardName: "shard_3",
						Hints: ShardHints{
							MinReplicaCount: 1,
							MaxReplicaCount: 10,
							MoveType:        MP_Cocurrent,
						},
					},
					{
						ShardName: "shard_4",
						Hints:     defaultHints,
					},
				},
			},
			description: "测试不同的移动策略配置",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseShardPlan(tt.input, defaultHints)

			if len(got.ShardLines) != len(tt.want.ShardLines) {
				t.Errorf("ShardLines length = %v, want %v", len(got.ShardLines), len(tt.want.ShardLines))
				return
			}

			for i, wantLine := range tt.want.ShardLines {
				gotLine := got.ShardLines[i]

				// 验证 ShardName
				if gotLine.ShardName != wantLine.ShardName {
					t.Errorf("ShardLines[%d].ShardName = %v, want %v", i, gotLine.ShardName, wantLine.ShardName)
				}

				// 验证 Hints
				if gotLine.Hints.MinReplicaCount != wantLine.Hints.MinReplicaCount {
					t.Errorf("ShardLines[%d].Hints.MinReplicaCount = %v, want %v", i, gotLine.Hints.MinReplicaCount, wantLine.Hints.MinReplicaCount)
				}
				if gotLine.Hints.MaxReplicaCount != wantLine.Hints.MaxReplicaCount {
					t.Errorf("ShardLines[%d].Hints.MaxReplicaCount = %v, want %v", i, gotLine.Hints.MaxReplicaCount, wantLine.Hints.MaxReplicaCount)
				}
				if gotLine.Hints.MoveType != wantLine.Hints.MoveType {
					t.Errorf("ShardLines[%d].Hints.MoveType = %v, want %v", i, gotLine.Hints.MoveType, wantLine.Hints.MoveType)
				}

				// 验证 CustomProperties
				if len(gotLine.CustomProperties) != len(wantLine.CustomProperties) {
					t.Errorf("ShardLines[%d].CustomProperties length = %v, want %v", i, len(gotLine.CustomProperties), len(wantLine.CustomProperties))
				}
				for k, v := range wantLine.CustomProperties {
					if gotLine.CustomProperties[k] != v {
						t.Errorf("ShardLines[%d].CustomProperties[%q] = %v, want %v", i, k, gotLine.CustomProperties[k], v)
					}
				}
			}
		})
	}
}
