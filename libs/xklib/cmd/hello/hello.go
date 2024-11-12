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

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
)

func main() {
	ctx := context.TODO()
	klogging.SetDefaultLogger(klogging.NewLogrusLogger(ctx).SetConfig(ctx, "debug", "json"))
	fmt.Println("hello")
	testKerror()
	// testKlogging()
}

func testKerror() {
	ke := kerror.Create("MyErr", "longer story").WithErrorCode(kerror.EC_INVALID_PARAMETER).With("app", "myApp")
	httpCode := ke.GetHttpErrorCode()
	msg := ke.Error()
	fmt.Printf("%d, %s\n", httpCode, msg)
}

func testKlogging() {
	klogging.Info(context.Background()).With("key", 1028).Log("serverSend", "")
}

func testTimer(ctx context.Context) {
	kcommon.TryCatchRun(ctx, func() {})
	// klogging.Info(context.Background()).With("key", 1028).Log("serverSend", "")
}
