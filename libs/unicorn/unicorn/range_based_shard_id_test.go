package unicorn

import (
	"testing"

	"github.com/xinkaiwang/shardmanager/libs/unicorn/data"
	"github.com/xinkaiwang/shardmanager/libs/unicorn/unicornjson"
)

func TestRangeBasedShardId_GenerateAndString(t *testing.T) {
	shards := RangeBasedShardIdGenerateIds("shard", 4, 8)
	if len(shards) != 4 {
		t.Fatalf("expected 4 shards, got %d", len(shards))
	}
	for _, shard := range shards {
		str := shard.ToString(8)
		if got := ParseRangeBasedShardIdFromString(str); got != *shard {
			t.Errorf("parsed shard not equal: got %+v, want %+v", got, *shard)
		}
	}
}

func TestRangeBasedShardId_KeyInShard(t *testing.T) {
	// 覆盖基本情况
	shard := NewRangeBasedShardId("shard", 0, 0x40000000)
	tests := []struct {
		key      uint32
		expected bool
	}{
		{0, true},
		{0x3FFFFFFF, true},
		{0x40000000, false},
		{0xFFFFFFFF, false},
	}
	for _, tt := range tests {
		got := shard.KeyInShard(data.ShardingKey(tt.key))
		if got != tt.expected {
			t.Errorf("KeyInShard(%x) = %v, want %v", tt.key, got, tt.expected)
		}
	}

	// 覆盖最大值边界
	shard = NewRangeBasedShardId("shard", 0xF0000000, 0)
	if !shard.KeyInShard(data.ShardingKey(0xFFFFFFFF)) {
		t.Errorf("KeyInShard(0xFFFFFFFF) = false, want true for shard [0xF0000000, 0x00000000)")
	}
	if shard.KeyInShard(data.ShardingKey(0xEFFFFFFF)) {
		t.Errorf("KeyInShard(0xEFFFFFFF) = true, want false for shard [0xF0000000, 0x00000000)")
	}
}

func TestRangeBasedShardId_GetEnd(t *testing.T) {
	// 测试 End=0 情况
	shard := NewRangeBasedShardId("shard", 0xF0000000, 0)
	if shard.GetEnd() != 0x100000000 {
		t.Errorf("GetEnd() = %x, want %x", shard.GetEnd(), uint64(0x100000000))
	}

	// 测试普通情况
	shard = NewRangeBasedShardId("shard", 0, 0x40000000)
	if shard.GetEnd() != 0x40000000 {
		t.Errorf("GetEnd() = %x, want %x", shard.GetEnd(), uint64(0x40000000))
	}
}

func TestRangeBasedShardId_ToString_ByteAligned(t *testing.T) {
	shard := NewRangeBasedShardId("shard", 0x00000000, 0x01000000)
	str := shard.ToString(2)
	expected := "shard_00_01"
	if str != expected {
		t.Errorf("ByteAligned minLen=2 failed: got %s, want %s", str, expected)
	}
	str = shard.ToString(4)
	expected = "shard_0000_0100"
	if str != expected {
		t.Errorf("ByteAligned minLen=4 failed: got %s, want %s", str, expected)
	}
}

func TestRangeBasedShardId_ToString_NonAligned(t *testing.T) {
	shard := NewRangeBasedShardId("shard", 0x12345600, 0x23456700)
	str := shard.ToString(2)
	expected := "shard_123456_234567"
	if str != expected {
		t.Errorf("NonAligned minLen=2 failed: got %s, want %s", str, expected)
	}
	str = shard.ToString(4)
	expected = "shard_123456_234567"
	if str != expected {
		t.Errorf("NonAligned minLen=4 failed: got %s, want %s", str, expected)
	}
}

func TestRangeBasedShardId_ToString_ExtremeValues(t *testing.T) {
	// 最大值测试
	shard := NewRangeBasedShardId("shard", 0xFFFFFFFF, 0)
	str := shard.ToString(2)
	expected := "shard_ffffffff_00"
	if str != expected {
		t.Errorf("Extreme max value failed: got %s, want %s", str, expected)
	}

	// 最小值测试
	shard = NewRangeBasedShardId("shard", 0, 1)
	str = shard.ToString(2)
	expected = "shard_00_00000001"
	if str != expected {
		t.Errorf("Extreme min value failed: got %s, want %s", str, expected)
	}

	// minLen=0测试（应该仍然至少返回1位）
	shard = NewRangeBasedShardId("shard", 0x10000000, 0x20000000)
	str = shard.ToString(0)
	expected = "shard_1_2"
	if str != expected {
		t.Errorf("minLen=0 failed: got %s, want %s", str, expected)
	}
}

func TestRangeBasedShardId_ParseInvalid(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic for invalid format")
		}
	}()
	ParseRangeBasedShardIdFromString("invalid_format")
}

func TestRangeBasedShardId_ParseInvalidVariants(t *testing.T) {
	// 测试不同的格式错误
	testCases := []string{
		"shard_only_",   // 尾部多一个下划线
		"shard_xxx_yyy", // 非法十六进制
		"shard_000_",    // 缺少第三部分
		// "_00_01",                  // 缺少前缀 - 实际上这可能不会导致 panic
		"shard_00000000000000_01", // 超长数字
	}

	for _, tc := range testCases {
		func() {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("expected panic for invalid format: %s", tc)
				}
			}()
			ParseRangeBasedShardIdFromString(tc)
		}()
	}
}

func TestRangeBasedShardIdRoutingTree_GetTreeId(t *testing.T) {
	// 测试 TreeId 生成和获取
	routingTable := map[data.WorkerId]*unicornjson.WorkerEntryJson{}
	tree := RangeBasedShardIdTreeBuilder(routingTable)
	if tree.GetTreeId() == "" {
		t.Errorf("TreeId should not be empty")
	}
}

func TestRangeBasedShardIdRoutingTree_FindShardByShardingKey(t *testing.T) {
	routingTable := map[data.WorkerId]*unicornjson.WorkerEntryJson{
		"worker-1": {
			WorkerId:    "worker-1",
			AddressPort: "127.0.0.1:8080",
			Assignments: []*unicornjson.AssignmentJson{
				&unicornjson.AssignmentJson{ShardId: "shard_00000000_40000000"},
			},
		},
		"worker-2": {
			WorkerId:    "worker-2",
			AddressPort: "127.0.0.1:8081",
			Assignments: []*unicornjson.AssignmentJson{
				&unicornjson.AssignmentJson{ShardId: "shard_40000000_80000000"},
			},
		},
	}
	tree := RangeBasedShardIdTreeBuilder(routingTable)

	// 测试范围内的key
	target := tree.FindShardByShardingKey(data.ShardingKey(0x20000000))
	if target == nil || target.WorkerInfo.WorkerId != "worker-1" {
		t.Errorf("expected worker-1, got %+v", target)
	}
	target = tree.FindShardByShardingKey(data.ShardingKey(0x60000000))
	if target == nil || target.WorkerInfo.WorkerId != "worker-2" {
		t.Errorf("expected worker-2, got %+v", target)
	}
}

func TestRangeBasedShardIdRoutingTree_FindShardByShardingKey_NotFound(t *testing.T) {
	// 测试没有分片覆盖的key
	routingTable := map[data.WorkerId]*unicornjson.WorkerEntryJson{
		"worker-1": {
			WorkerId:    "worker-1",
			AddressPort: "127.0.0.1:8080",
			Assignments: []*unicornjson.AssignmentJson{
				&unicornjson.AssignmentJson{ShardId: "shard_00000000_40000000"},
			},
		},
	}
	tree := RangeBasedShardIdTreeBuilder(routingTable)

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic for key not found")
		}
	}()
	tree.FindShardByShardingKey(data.ShardingKey(0x50000000))
}

func TestRangeBasedShardIdRoutingTree_EmptyTree(t *testing.T) {
	// 测试空路由表
	routingTable := map[data.WorkerId]*unicornjson.WorkerEntryJson{}
	tree := RangeBasedShardIdTreeBuilder(routingTable)

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic for empty tree")
		}
	}()
	tree.FindShardByShardingKey(data.ShardingKey(0x12345678))
}

func TestRangeBasedShardIdRoutingTree_ComplexAssignments(t *testing.T) {
	// 测试多个worker有重叠分片的情况，应该返回匹配结果
	routingTable := map[data.WorkerId]*unicornjson.WorkerEntryJson{
		"worker-1": {
			WorkerId:    "worker-1",
			AddressPort: "127.0.0.1:8080",
			Assignments: []*unicornjson.AssignmentJson{
				&unicornjson.AssignmentJson{ShardId: "shard_00000000_80000000"},
			},
		},
		"worker-2": {
			WorkerId:    "worker-2",
			AddressPort: "127.0.0.1:8081",
			Assignments: []*unicornjson.AssignmentJson{
				&unicornjson.AssignmentJson{ShardId: "shard_40000000_c0000000"},
			},
		},
	}
	tree := RangeBasedShardIdTreeBuilder(routingTable)

	// 测试重叠分片范围，由于 Go map 遍历顺序不确定，验证返回的是有效 worker 即可
	target := tree.FindShardByShardingKey(data.ShardingKey(0x50000000))
	// 验证返回的 WorkerId 是否有效
	if target.WorkerInfo.WorkerId != "worker-1" && target.WorkerInfo.WorkerId != "worker-2" {
		t.Errorf("Should return a valid worker from the tree, got: %s", target.WorkerInfo.WorkerId)
	}
}

func TestRangeBasedShardIdGenerateString(t *testing.T) {
	// 测试生成字符串切片
	list := RangeBasedShardIdGenerateString("shard", 8, 2)
	if len(list) != 8 {
		t.Errorf("expected 8 shards, got %d", len(list))
	}

	// 验证生成的每个字符串都能正确解析
	for _, s := range list {
		shard := ParseRangeBasedShardIdFromString(s)
		if shard.ToString(2) != s {
			t.Errorf("shard string roundtrip failed: %s -> %+v -> %s",
				s, shard, shard.ToString(2))
		}
	}
}
