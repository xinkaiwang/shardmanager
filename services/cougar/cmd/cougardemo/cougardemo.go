package main

import "fmt"

// 版本信息，通过 ldflags 在构建时注入
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

func main() {
	// 显示版本信息
	fmt.Printf("Cougar Demo %s (Commit: %s, Built: %s)\n", Version, GitCommit, BuildTime)
	fmt.Println("Hello, World!")
}
