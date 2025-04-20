package core

import (
	"context"
	"testing"
)

func TestAssembleAssignSolver(t *testing.T) {
	ctx := context.Background()
	// klogging.SetDefaultLogger(klogging.NewLogrusLogger(ctx).SetConfig(ctx, "debug", "text"))

	// 配置测试环境
	setup := NewFakeTimeTestSetup(t)
	setup.SetupBasicConfig(ctx)
	t.Logf("测试环境已配置")

	fn := func() {
		// Step 1: 创建 worker-1 eph
		workerFullId, _ := setup.CreateAndSetWorkerEph(t, "worker-1", "session-1", "localhost:8080")

	}
}
