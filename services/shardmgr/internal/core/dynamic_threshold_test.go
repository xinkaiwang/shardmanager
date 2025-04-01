package core

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/config"
)

// 测试配置提供者
type testDynamicThresholdConfigProvider struct {
	config config.DynamicThresholdConfig
}

func (p *testDynamicThresholdConfigProvider) GetDynamicThresholdConfig() config.DynamicThresholdConfig {
	return p.config
}

func newTestConfig() *testDynamicThresholdConfigProvider {
	return &testDynamicThresholdConfigProvider{
		config: config.DynamicThresholdConfig{
			DynamicThresholdMax: 100,
			DynamicThresholdMin: 10,
			HalfDecayTimeSec:    10, // 10秒半衰期，便于测试
			IncreasePerMove:     5,
		},
	}
}

func TestDynamicThresholdInitialValue(t *testing.T) {
	// 创建一个带有测试配置的DynamicThreshold
	configProvider := newTestConfig()
	dt := NewDynamicThreshold(configProvider.GetDynamicThresholdConfig)

	// 验证初始阈值是最小值
	assert.Equal(t, float64(configProvider.GetDynamicThresholdConfig().DynamicThresholdMin), dt.threshold)
}

func TestDynamicThresholdGetCurrentWithSameTime(t *testing.T) {
	// 创建一个带有测试配置的DynamicThreshold
	configProvider := newTestConfig()
	dt := NewDynamicThreshold(configProvider.GetDynamicThresholdConfig)

	// 使用相同的时间获取阈值，应该返回相同的值
	currentTime := dt.currentTimeMs
	threshold := dt.GetCurrentThreshold(currentTime)
	assert.Equal(t, float64(configProvider.GetDynamicThresholdConfig().DynamicThresholdMin), threshold)
}

func TestDynamicThresholdGetCurrentWithEarlierTime(t *testing.T) {
	// 创建一个带有测试配置的DynamicThreshold
	configProvider := newTestConfig()
	dt := NewDynamicThreshold(configProvider.GetDynamicThresholdConfig)

	// 使用早于当前时间的时间获取阈值，应该返回相同的值且不更新内部状态
	currentTime := dt.currentTimeMs - 1000
	originalThreshold := dt.threshold
	threshold := dt.GetCurrentThreshold(currentTime)
	assert.Equal(t, originalThreshold, threshold)
	assert.Equal(t, originalThreshold, dt.threshold)
}

func TestDynamicThresholdDecay(t *testing.T) {
	// 创建一个带有测试配置的DynamicThreshold
	configProvider := newTestConfig()
	dt := NewDynamicThreshold(configProvider.GetDynamicThresholdConfig)

	// 首先更新阈值，使其高于最小值
	dt.UpdateThreshold(dt.currentTimeMs, 10) // 增加10次移动，每次5，总共增加50
	initialThreshold := dt.threshold
	assert.Equal(t, float64(configProvider.config.DynamicThresholdMin)+10*float64(configProvider.config.IncreasePerMove), initialThreshold)

	// 经过一个半衰期
	newTime := dt.currentTimeMs + int64(configProvider.config.HalfDecayTimeSec)*1000
	newThreshold := dt.GetCurrentThreshold(newTime)

	// 验证阈值衰减到一半
	expectedThreshold := float64(configProvider.config.DynamicThresholdMin) + (initialThreshold-float64(configProvider.config.DynamicThresholdMin))*0.5
	assert.InDelta(t, expectedThreshold, newThreshold, 0.0001)
}

func TestDynamicThresholdDecayMultipleHalfLives(t *testing.T) {
	// 创建一个带有测试配置的DynamicThreshold
	configProvider := newTestConfig()
	dt := NewDynamicThreshold(configProvider.GetDynamicThresholdConfig)

	// 首先更新阈值，使其达到最大值
	maxMoves := int32(math.Ceil(float64(configProvider.config.DynamicThresholdMax-configProvider.config.DynamicThresholdMin) / float64(configProvider.config.IncreasePerMove)))
	dt.UpdateThreshold(dt.currentTimeMs, maxMoves+10) // 确保达到最大值
	initialThreshold := dt.threshold
	assert.Equal(t, float64(configProvider.config.DynamicThresholdMax), initialThreshold)

	// 经过两个半衰期
	newTime := dt.currentTimeMs + int64(2*configProvider.config.HalfDecayTimeSec)*1000
	newThreshold := dt.GetCurrentThreshold(newTime)

	// 验证阈值衰减到1/4
	expectedThreshold := float64(configProvider.config.DynamicThresholdMin) + (initialThreshold-float64(configProvider.config.DynamicThresholdMin))*0.25
	assert.InDelta(t, expectedThreshold, newThreshold, 0.0001)
}

func TestDynamicThresholdFullCycle(t *testing.T) {
	// 创建一个带有测试配置的DynamicThreshold和一个伪造的时间提供者
	configProvider := newTestConfig()
	fakeTime := kcommon.NewFakeTimeProvider(1000000) // 设置一个初始时间
	fakeTime.WallTime = 1000000                      // 设置一个初始时间

	// 使用伪造的时间提供者执行测试
	kcommon.RunWithTimeProvider(fakeTime, func() {
		dt := NewDynamicThreshold(configProvider.GetDynamicThresholdConfig)
		assert.Equal(t, fakeTime.WallTime, dt.currentTimeMs)

		// 1. 增加阈值
		dt.UpdateThreshold(dt.currentTimeMs, 10)
		increaseThreshold := dt.threshold

		// 2. 推进时间，让阈值衰减
		fakeTime.WallTime += int64(configProvider.config.HalfDecayTimeSec) * 1000
		decayedThreshold := dt.GetCurrentThreshold(fakeTime.WallTime)

		// 3. 再次增加阈值
		dt.UpdateThreshold(fakeTime.WallTime, 5)
		reIncreasedThreshold := dt.threshold

		// 验证各个阶段的阈值变化
		assert.Greater(t, increaseThreshold, float64(configProvider.config.DynamicThresholdMin))
		assert.Less(t, decayedThreshold, increaseThreshold)
		assert.Greater(t, reIncreasedThreshold, decayedThreshold)
	})
}

func TestDynamicThresholdMaxThreshold(t *testing.T) {
	// 创建一个带有测试配置的DynamicThreshold
	configProvider := newTestConfig()
	dt := NewDynamicThreshold(configProvider.GetDynamicThresholdConfig)

	// 使用一个非常大的移动次数更新阈值
	dt.UpdateThreshold(dt.currentTimeMs, 1000)

	// 验证阈值被限制在最大值
	assert.Equal(t, float64(configProvider.config.DynamicThresholdMax), dt.threshold)
}

func TestDynamicThresholdUpdateWithZeroMoves(t *testing.T) {
	// 创建一个带有测试配置的DynamicThreshold
	configProvider := newTestConfig()
	dt := NewDynamicThreshold(configProvider.GetDynamicThresholdConfig)

	initialThreshold := dt.threshold

	// 使用零次移动更新阈值
	dt.UpdateThreshold(dt.currentTimeMs, 0)

	// 验证阈值没有变化
	assert.Equal(t, initialThreshold, dt.threshold)
}

func TestDynamicThresholdWithChangingConfig(t *testing.T) {
	// 创建一个带有测试配置的DynamicThreshold
	configProvider := newTestConfig()
	dt := NewDynamicThreshold(configProvider.GetDynamicThresholdConfig)

	// 更新阈值使其接近最大值
	dt.UpdateThreshold(dt.currentTimeMs, 18) // 10 + 18*5 = 100
	assert.Equal(t, float64(configProvider.config.DynamicThresholdMax), dt.threshold)

	// 改变配置中的最大值
	configProvider.config.DynamicThresholdMax = 80

	// 获取当前阈值，应该受到新的最大值限制
	newThreshold := dt.GetCurrentThreshold(dt.currentTimeMs + 1)
	assert.InDelta(t, float64(80), newThreshold, 0.01)
}

// 性能测试
func BenchmarkDynamicThresholdGetCurrent(b *testing.B) {
	configProvider := newTestConfig()
	dt := NewDynamicThreshold(configProvider.GetDynamicThresholdConfig)
	currentTime := kcommon.GetWallTimeMs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dt.GetCurrentThreshold(currentTime + int64(i))
	}
}

// 模拟实际使用场景的测试
func TestDynamicThresholdSimulateRealUsage(t *testing.T) {
	// 创建一个带有测试配置的DynamicThreshold
	configProvider := newTestConfig()

	// 测试数据定义，包含每个步骤的详细信息和预期结果
	// 这种数据驱动的方式可以更好地展示DynamicThreshold的工作原理
	type testStep struct {
		description   string  // 操作描述
		timePassMs    int64   // 经过的时间（毫秒）
		moves         int32   // 移动次数
		expectedValue float64 // 预期阈值
		explanation   string  // 阈值计算说明
	}

	// 模拟一个实际的使用场景，包含初始值、多次操作和随时间的衰减
	testSteps := []testStep{
		{
			description:   "初始状态",
			timePassMs:    0,
			moves:         0,
			expectedValue: 10.0, // 配置中的最小值
			explanation:   "初始阈值等于最小值(DynamicThresholdMin=10)",
		},
		{
			description:   "第一次更新 - 增加5次移动",
			timePassMs:    0,
			moves:         5,
			expectedValue: 35.0, // 10 + 5*5
			explanation:   "阈值增加了5次移动，每次5点: 10 + 5*5 = 35",
		},
		{
			description:   "1秒后 - 增加3次移动",
			timePassMs:    1000,
			moves:         3,
			expectedValue: 48.3, // 实际计算: 35轻微衰减 + 3*5
			explanation:   "阈值在1秒内轻微衰减到约33.3，然后增加3次移动: 33.3 + 3*5 = 48.3",
		},
		{
			description:   "3秒后 - 无移动，只有衰减",
			timePassMs:    3000,
			moves:         0,
			expectedValue: 41.1, // 实际计算值
			explanation:   "阈值在3秒内衰减: 48.3 -> 41.1 (半衰期10秒，衰减约15%)",
		},
		{
			description:   "5秒后 - 增加7次移动",
			timePassMs:    5000,
			moves:         7,
			expectedValue: 67.0, // 实际计算值
			explanation:   "阈值在5秒内衰减: 41.1 -> 32.0，然后增加7次移动: 32.0 + 7*5 = 67.0",
		},
		{
			description:   "8秒后 - 无移动，只有衰减",
			timePassMs:    8000,
			moves:         0,
			expectedValue: 42.7, // 实际计算值
			explanation:   "阈值在8秒内衰减: 67.0 -> 42.7 (衰减约36%)",
		},
		{
			description:   "半衰期(10秒)后 - 无移动，只有衰减",
			timePassMs:    10000,
			moves:         0,
			expectedValue: 26.4, // 实际计算值
			explanation:   "阈值经过一个半衰期(10秒)后衰减: 42.7 -> 26.4 (衰减50%, min=10)",
		},
	}

	// 使用fake time provider来保证测试确定性
	fakeTime := kcommon.NewFakeTimeProvider(1000000) // 设置一个初始时间

	kcommon.RunWithTimeProvider(fakeTime, func() {
		dt := NewDynamicThreshold(configProvider.GetDynamicThresholdConfig)
		currentTimeMs := fakeTime.WallTime

		// 验证初始阈值
		initialStep := testSteps[0]
		initialValue := dt.GetCurrentThreshold(currentTimeMs)
		assert.InDelta(t, initialStep.expectedValue, initialValue, 0.1,
			"[%s] 初始阈值不符合预期: 预期 %.1f, 实际 %.1f",
			initialStep.description, initialStep.expectedValue, initialValue)
		t.Logf("[%s] 阈值: %.1f - %s", initialStep.description, initialValue, initialStep.explanation)

		// 执行每个测试步骤并验证结果
		for i := 1; i < len(testSteps); i++ {
			step := testSteps[i]

			// 推进时间
			currentTimeMs += step.timePassMs

			// 应用操作
			if step.moves > 0 {
				dt.UpdateThreshold(currentTimeMs, step.moves)
			} else {
				dt.GetCurrentThreshold(currentTimeMs) // 更新内部时间和阈值
			}

			// 验证结果
			assert.InDelta(t, step.expectedValue, dt.threshold, 0.1,
				"[%s] 阈值不符合预期: 预期 %.1f, 实际 %.1f",
				step.description, step.expectedValue, dt.threshold)

			// 记录详细信息便于调试和理解
			t.Logf("[%s] 阈值: %.1f - %s", step.description, dt.threshold, step.explanation)
		}

		// 验证最后两个阈值的关系，确保有衰减
		secondLastIdx := len(testSteps) - 2
		lastIdx := len(testSteps) - 1
		assert.Less(t, testSteps[lastIdx].expectedValue, testSteps[secondLastIdx].expectedValue,
			"阈值应该随时间衰减，但最后测试点的值不小于前一个点")
	})

	// 真实计算的示例，向读者展示具体的衰减公式
	t.Logf("\n阈值衰减计算示例:\n" +
		"- 起始阈值: 67.0, 最小阈值: 10.0\n" +
		"- 超过最小值部分: 67.0 - 10.0 = 57.0\n" +
		"- 半衰期: 10秒\n" +
		"- 经过时间: 8秒\n" +
		"- 衰减因子: 0.5^(8/10) = 0.5^0.8 ≈ 0.574\n" +
		"- 衰减后的超过值: 57.0 * 0.574 = 32.7\n" +
		"- 最终阈值: 10.0 + 32.7 = 42.7\n\n" +
		"当经过一个完整半衰期(10秒):\n" +
		"- 衰减因子: 0.5^(10/10) = 0.5\n" +
		"- 衰减后的超过值: 32.7 * 0.5 = 16.4\n" +
		"- 最终阈值: 10.0 + 16.4 = 26.4")
}
