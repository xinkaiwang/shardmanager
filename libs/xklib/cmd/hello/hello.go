/*
Copyright 2024 Xinkai Wang.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
)

func main() {
	ctx := context.TODO()
	
	// Initialize OpenTelemetry for trace propagation
	klogging.InitOpenTelemetry()
	
	// Create and configure slog handler
	handler := klogging.NewHandler(&klogging.HandlerOptions{
		Level:  klogging.LevelDebug,
		Format: "json",
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)
	
	fmt.Println("hello")
	// testKerror()
	// testSlog()
	testLoggingCtx(ctx)
}

func testKerror() {
	ke := kerror.Create("MyErr", "longer story").WithErrorCode(kerror.EC_INVALID_PARAMETER).With("app", "myApp")
	httpCode := ke.GetHttpErrorCode()
	msg := ke.Error()
	fmt.Printf("%d, %s\n", httpCode, msg)
}

func testSlog() {
	slog.InfoContext(context.Background(), "server send",
		slog.String("event", "serverSend"),
		slog.Int("key", 1028))
}

func testTimer(ctx context.Context) {
	kcommon.TryCatchRun(ctx, func() {})
}

func testLoggingCtx(ctx context.Context) {
	// log with context
	slog.InfoContext(ctx, "server send",
		slog.String("event", "serverSend"),
		slog.Int("key", 1028))
	
	// Note: OpenTelemetry handles trace IDs automatically
	// No need to manually embed trace IDs like before
	// Trace context is automatically propagated through ctx
	
	slog.InfoContext(ctx, "server send with auto trace",
		slog.String("event", "serverSend"),
		slog.Int("key", 1028))
}
