package unicorn

import (
	"context"
	"strconv"
	"strings"

	"github.com/xinkaiwang/shardmanager/libs/unicorn/data"
	"github.com/xinkaiwang/shardmanager/libs/unicorn/unicornjson"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
)

/*
RangeBasedShardId looks like this:
	<shardPfx>_<start>_<end>

for example:
shard_0000_2000
shard_2000_4000
shard_4000_6000
...
shard_E000_F000
shard_F000_0000
*/

// RangeBasedShardId implements ShardIdStruct interface
type RangeBasedShardId struct {
	ShardPfx string
	Start    uint32 // start is inclusive
	End      uint32 // end is exclusive
}

func NewRangeBasedShardId(shardPfx string, start, end uint64) RangeBasedShardId {
	return RangeBasedShardId{
		ShardPfx: shardPfx,
		Start:    uint32(start),
		End:      uint32(end),
	}
}

func (rbs RangeBasedShardId) ToString(minLen int) string {
	return rbs.ShardPfx + "_" + uint32ToString(rbs.Start, minLen) + "_" + uint32ToString(rbs.End, minLen)
}

func uint32ToString(num uint32, minLen int) string {
	str := strconv.FormatUint(uint64(num), 16)
	for len(str) < 8 { // step 1: patch to full 8 digits
		str = "0" + str
	}
	for len(str) > minLen && str[len(str)-1] == '0' { // step 2: remove trailing zeros
		str = str[:len(str)-1]
	}
	return str
}

func (rbs RangeBasedShardId) GetStart() uint64 {
	return uint64(rbs.Start)
}
func (rbs RangeBasedShardId) GetEnd() uint64 {
	if rbs.End == 0 {
		return uint64(1) << 32
	}
	return uint64(rbs.End)
}

func ParseRangeBasedShardIdFromString(str string) RangeBasedShardId {
	parts := strings.Split(str, "_")
	if len(parts) != 3 {
		ke := kerror.Create("InvalidFormat", "invalid format for RangeBasedShardId").With("str", str)
		panic(ke)
	}
	start, err := strconv.ParseUint(parts[1], 16, 32)
	if err != nil {
		ke := kerror.Wrap(err, "ParseError", "failed to parse start", false)
		panic(ke)
	}
	start <<= (32 - len(parts[1])*4)
	end, err := strconv.ParseUint(parts[2], 16, 32)
	if err != nil {
		ke := kerror.Wrap(err, "ParseError", "failed to parse end", false)
		panic(ke)
	}
	end <<= (32 - len(parts[2])*4)
	return NewRangeBasedShardId(parts[0], start, end)
}

// implements ShardIdStruct interface
func (rbs RangeBasedShardId) KeyInShard(key data.ShardingKey) bool {
	val := uint64(key)
	if val >= rbs.GetEnd() {
		return false
	}
	if val < rbs.GetStart() {
		return false
	}
	return true
}

// RangeBasedShardIdRoutingTree implements RoutingTree interface
type RangeBasedShardIdRoutingTree struct {
	TreeId  string
	Workers map[data.WorkerId]*RbsWorkerEntry
}

func (rbs RangeBasedShardIdRoutingTree) GetTreeId() string {
	return rbs.TreeId
}

// FindShardByShardingKey will panic if not found
func (rbs RangeBasedShardIdRoutingTree) FindShardByShardingKey(shardingKey data.ShardingKey) *RoutingTarget {
	// return the fisrst worker that matches the sharding key
	for _, worker := range rbs.Workers {
		for _, assignment := range worker.Assignments {
			if assignment.RangeBasedShardId.KeyInShard(shardingKey) {
				return &RoutingTarget{
					ShardId:    assignment.ShardId,
					WorkerInfo: &worker.WorkerInfo,
				}
			}
		}
	}
	ke := kerror.Create("NotFound", "sharding key not found").With("shardingKey", shardingKey)
	panic(ke)
}

// helper function to convert ShardId to/from RangeBasedShardId
func RangeBasedShardIdTreeBuilder(routingTable map[data.WorkerId]*unicornjson.WorkerEntryJson) *RangeBasedShardIdRoutingTree {
	tree := &RangeBasedShardIdRoutingTree{
		TreeId:  kcommon.RandomString(context.Background(), 8),
		Workers: make(map[data.WorkerId]*RbsWorkerEntry),
	}
	for _, entry := range routingTable {
		workerId := data.WorkerId(entry.WorkerId)
		addressPort := entry.AddressPort
		assignments := make(map[data.ShardId]*RbsAssignment)
		for _, assignment := range entry.Assignments {
			shardId := data.ShardId(assignment.ShardId)
			rangeBasedShardId := ParseRangeBasedShardIdFromString(assignment.ShardId)
			assignments[shardId] = &RbsAssignment{
				ShardId:           shardId,
				RangeBasedShardId: rangeBasedShardId,
			}
		}
		tree.Workers[workerId] = &RbsWorkerEntry{
			WorkerInfo: WorkerInfo{
				WorkerId:    workerId,
				AddressPort: addressPort,
			},
			Assignments: assignments,
		}
	}
	return tree
}

type WorkerInfo struct {
	WorkerId    data.WorkerId
	AddressPort string
}

type RbsWorkerEntry struct {
	WorkerInfo
	Assignments map[data.ShardId]*RbsAssignment
}

type RbsAssignment struct {
	ShardId           data.ShardId
	RangeBasedShardId RangeBasedShardId
}

func RangeBasedShardIdGenerateIds(shardPfx string, count int, strLen int) []*RangeBasedShardId {
	var shards []*RangeBasedShardId
	total := uint64(1 << 32)
	start := uint64(0)
	for i := 1; i <= count; i++ {
		end := (total * uint64(i)) / uint64(count)
		shardId := NewRangeBasedShardId(shardPfx, start, end)
		shards = append(shards, &shardId)
		start = end
	}
	return shards
}

func RangeBasedShardIdGenerateString(shardPfx string, count int, strLen int) []string {
	shards := RangeBasedShardIdGenerateIds(shardPfx, count, strLen)
	var list []string
	for _, shard := range shards {
		list = append(list, shard.ToString(strLen))
	}
	return list
}
