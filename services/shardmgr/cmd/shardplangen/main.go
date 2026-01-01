package main

import (
	"fmt"
	"strconv"
	"strings"
)

var (
	shardPrefix = "shard"
	shardCount  = 256
	minLenth    = 4
)

// Generate shard plan, which is a list of shard names.
// prefix_start_end. [start, end) left close, right open
func main() {
	var start, end int64 = 0, 10000
	for i := 0; i < shardCount; i++ {
		start = int64((1 << 32) / shardCount * i)
		end = int64((1 << 32) / shardCount * (i + 1))
		fmt.Println(formatShardName(start, end))
	}
}

func formatShardName(start, end int64) string {
	return fmt.Sprintf("%s_%s_%s", shardPrefix, formatPos(start), formatPos(end))
}

func formatPos(pos int64) string {
	str := strconv.FormatUint(uint64(pos), 16)
	str = strings.ToUpper(str)
	if len(str) > 8 {
		str = str[len(str)-8:]
	}
	if len(str) < 8 {
		str = fmt.Sprintf("%08s", str)
	}
	for len(str) > minLenth && str[len(str)-1] == '0' {
		str = str[0 : len(str)-1]
	}
	return str
}
