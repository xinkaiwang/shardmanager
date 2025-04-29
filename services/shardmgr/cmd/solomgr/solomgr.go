package main

import (
	"context"
	"fmt"
	"os"

	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/solo"
)

func main() {
	ctx := context.Background()
	fmt.Println("Hello, World!")
	soloMgr := solo.NewSoloManager(ctx, "pod1")
	go func() {
		<-soloMgr.ChLockLost
		fmt.Println("Lock lost, exiting...")
		os.Exit(1)
	}()
	soloMgr.PutNode("test")
	// read line from stdin
	var input string
	fmt.Scanln(&input)
	soloMgr.Close(ctx)
}
