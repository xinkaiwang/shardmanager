package common

import (
	"os"
	"testing"
	"time"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
)

func TestMapEqual(t *testing.T) {
	tests := []struct {
		name string
		m1   map[string]string
		m2   map[string]string
		want bool
	}{
		{
			name: "空map相等",
			m1:   map[string]string{},
			m2:   map[string]string{},
			want: true,
		},
		{
			name: "nil map相等",
			m1:   nil,
			m2:   nil,
			want: true,
		},
		{
			name: "一个nil一个空map不相等",
			m1:   nil,
			m2:   map[string]string{},
			want: false,
		},
		{
			name: "相同内容的map相等",
			m1:   map[string]string{"a": "1", "b": "2"},
			m2:   map[string]string{"b": "2", "a": "1"},
			want: true,
		},
		{
			name: "不同内容的map不相等",
			m1:   map[string]string{"a": "1", "b": "2"},
			m2:   map[string]string{"a": "1", "b": "3"},
			want: false,
		},
		{
			name: "不同大小的map不相等",
			m1:   map[string]string{"a": "1", "b": "2"},
			m2:   map[string]string{"a": "1"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MapEqual(tt.m1, tt.m2); got != tt.want {
				t.Errorf("MapEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetEnvInt(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue int
		envValue     string
		want         int
	}{
		{
			name:         "环境变量不存在时返回默认值",
			key:          "TEST_ENV_NOT_EXISTS",
			defaultValue: 42,
			envValue:     "",
			want:         42,
		},
		{
			name:         "环境变量存在且有效时返回环境变量值",
			key:          "TEST_ENV_VALID",
			defaultValue: 42,
			envValue:     "100",
			want:         100,
		},
		{
			name:         "环境变量存在但无效时返回默认值",
			key:          "TEST_ENV_INVALID",
			defaultValue: 42,
			envValue:     "not_a_number",
			want:         42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}

			if got := GetEnvInt(tt.key, tt.defaultValue); got != tt.want {
				t.Errorf("GetEnvInt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetVersion(t *testing.T) {
	// 由于 version 是包级变量，我们只能测试它返回的是否是预期的默认值
	if got := GetVersion(); got != "unknown" {
		t.Errorf("GetVersion() = %v, want %v", got, "unknown")
	}
}

func TestGetSessionId(t *testing.T) {
	// 测试多次调用返回相同的 sessionId
	first := GetSessionId()
	if first == "" {
		t.Error("GetSessionId() returned empty string")
	}

	second := GetSessionId()
	if second != first {
		t.Errorf("GetSessionId() returned different values: first = %v, second = %v", first, second)
	}

	// 验证 sessionId 的长度是否为 8
	if len(first) != 8 {
		t.Errorf("GetSessionId() returned string of length %v, want 8", len(first))
	}
}

func TestGetStartTimeMs(t *testing.T) {
	// 获取当前时间作为参考
	now := kcommon.GetWallTimeMs()

	// 获取启动时间
	startTime := GetStartTimeMs()

	// 验证启动时间是否在合理范围内（不早于编译时间，不晚于当前时间）
	if startTime > now {
		t.Errorf("GetStartTimeMs() = %v, which is in the future (now = %v)", startTime, now)
	}

	// 启动时间应该在最近 5 分钟内（考虑到测试环境的时间差异）
	fiveMinutesAgo := now - int64(5*time.Minute/time.Millisecond)
	if startTime < fiveMinutesAgo {
		t.Errorf("GetStartTimeMs() = %v, which is too old (5 minutes ago = %v)", startTime, fiveMinutesAgo)
	}
}
