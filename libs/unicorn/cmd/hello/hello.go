package main

import (
	"fmt"
	"strings"

	"github.com/xinkaiwang/shardmanager/libs/unicorn/unicorn"
)

func main() {
	// entry := unicornjson.NewWorkerEntryJson("unicorn-worker-75fffc88f9-fkbcm", "127.0.0.1:8080", "init")
	// fmt.Printf("Hello, world.\n" + entry.ToJson())

	list := unicorn.RangeBasedShardIdGenerateString("shard", 1024, 3)
	fmt.Println(strings.Join(list, "\n"))
}
