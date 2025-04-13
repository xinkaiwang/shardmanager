package main

import (
	"fmt"

	"github.com/xinkaiwang/shardmanager/services/unicorn/unicornjson"
)

func main() {
	entry := unicornjson.NewWorkerEntryJson("unicorn-worker-75fffc88f9-fkbcm", "127.0.0.1:8080", "init")
	fmt.Printf("Hello, world.\n" + entry.ToJson())
}
