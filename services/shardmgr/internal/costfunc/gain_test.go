package costfunc

import (
	"testing"
)

func TestCost_IsLowerThan(t *testing.T) {
	tests := []struct {
		name     string
		cost     Cost
		other    Cost
		expected bool
	}{
		{
			name:     "硬分数更低",
			cost:     Cost{HardScore: 1, SoftScore: 2.0},
			other:    Cost{HardScore: 2, SoftScore: 1.0},
			expected: true,
		},
		{
			name:     "硬分数更高",
			cost:     Cost{HardScore: 2, SoftScore: 1.0},
			other:    Cost{HardScore: 1, SoftScore: 2.0},
			expected: false,
		},
		{
			name:     "硬分数相等，软分数更低",
			cost:     Cost{HardScore: 1, SoftScore: 1.0},
			other:    Cost{HardScore: 1, SoftScore: 2.0},
			expected: true,
		},
		{
			name:     "硬分数相等，软分数更高",
			cost:     Cost{HardScore: 1, SoftScore: 2.0},
			other:    Cost{HardScore: 1, SoftScore: 1.0},
			expected: false,
		},
		{
			name:     "完全相等",
			cost:     Cost{HardScore: 1, SoftScore: 1.0},
			other:    Cost{HardScore: 1, SoftScore: 1.0},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cost.IsLowerThan(tt.other); got != tt.expected {
				t.Errorf("Cost.IsLowerThan() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCost_IsEqualTo(t *testing.T) {
	tests := []struct {
		name     string
		cost     Cost
		other    Cost
		expected bool
	}{
		{
			name:     "完全相等",
			cost:     Cost{HardScore: 1, SoftScore: 1.0},
			other:    Cost{HardScore: 1, SoftScore: 1.0},
			expected: true,
		},
		{
			name:     "硬分数不同",
			cost:     Cost{HardScore: 1, SoftScore: 1.0},
			other:    Cost{HardScore: 2, SoftScore: 1.0},
			expected: false,
		},
		{
			name:     "软分数不同",
			cost:     Cost{HardScore: 1, SoftScore: 1.0},
			other:    Cost{HardScore: 1, SoftScore: 2.0},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cost.IsEqualTo(tt.other); got != tt.expected {
				t.Errorf("Cost.IsEqualTo() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCost_IsGreaterThan(t *testing.T) {
	tests := []struct {
		name     string
		cost     Cost
		other    Cost
		expected bool
	}{
		{
			name:     "硬分数更高",
			cost:     Cost{HardScore: 2, SoftScore: 1.0},
			other:    Cost{HardScore: 1, SoftScore: 2.0},
			expected: true,
		},
		{
			name:     "硬分数更低",
			cost:     Cost{HardScore: 1, SoftScore: 2.0},
			other:    Cost{HardScore: 2, SoftScore: 1.0},
			expected: false,
		},
		{
			name:     "硬分数相等，软分数更高",
			cost:     Cost{HardScore: 1, SoftScore: 2.0},
			other:    Cost{HardScore: 1, SoftScore: 1.0},
			expected: true,
		},
		{
			name:     "硬分数相等，软分数更低",
			cost:     Cost{HardScore: 1, SoftScore: 1.0},
			other:    Cost{HardScore: 1, SoftScore: 2.0},
			expected: false,
		},
		{
			name:     "完全相等",
			cost:     Cost{HardScore: 1, SoftScore: 1.0},
			other:    Cost{HardScore: 1, SoftScore: 1.0},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cost.IsGreaterThan(tt.other); got != tt.expected {
				t.Errorf("Cost.IsGreaterThan() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGain_IsGreaterThan(t *testing.T) {
	tests := []struct {
		name     string
		gain     Gain
		other    Gain
		expected bool
	}{
		{
			name:     "硬分数更高",
			gain:     Gain{HardScore: 2, SoftScore: 1.0},
			other:    Gain{HardScore: 1, SoftScore: 2.0},
			expected: true,
		},
		{
			name:     "硬分数更低",
			gain:     Gain{HardScore: 1, SoftScore: 2.0},
			other:    Gain{HardScore: 2, SoftScore: 1.0},
			expected: false,
		},
		{
			name:     "硬分数相等，软分数更高",
			gain:     Gain{HardScore: 1, SoftScore: 2.0},
			other:    Gain{HardScore: 1, SoftScore: 1.0},
			expected: true,
		},
		{
			name:     "硬分数相等，软分数更低",
			gain:     Gain{HardScore: 1, SoftScore: 1.0},
			other:    Gain{HardScore: 1, SoftScore: 2.0},
			expected: false,
		},
		{
			name:     "完全相等",
			gain:     Gain{HardScore: 1, SoftScore: 1.0},
			other:    Gain{HardScore: 1, SoftScore: 1.0},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.gain.IsGreaterThan(tt.other); got != tt.expected {
				t.Errorf("Gain.IsGreaterThan() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGain_IsLowerThan(t *testing.T) {
	tests := []struct {
		name     string
		gain     Gain
		other    Gain
		expected bool
	}{
		{
			name:     "硬分数更低",
			gain:     Gain{HardScore: 1, SoftScore: 2.0},
			other:    Gain{HardScore: 2, SoftScore: 1.0},
			expected: true,
		},
		{
			name:     "硬分数更高",
			gain:     Gain{HardScore: 2, SoftScore: 1.0},
			other:    Gain{HardScore: 1, SoftScore: 2.0},
			expected: false,
		},
		{
			name:     "硬分数相等，软分数更低",
			gain:     Gain{HardScore: 1, SoftScore: 1.0},
			other:    Gain{HardScore: 1, SoftScore: 2.0},
			expected: true,
		},
		{
			name:     "硬分数相等，软分数更高",
			gain:     Gain{HardScore: 1, SoftScore: 2.0},
			other:    Gain{HardScore: 1, SoftScore: 1.0},
			expected: false,
		},
		{
			name:     "完全相等",
			gain:     Gain{HardScore: 1, SoftScore: 1.0},
			other:    Gain{HardScore: 1, SoftScore: 1.0},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.gain.IsLowerThan(tt.other); got != tt.expected {
				t.Errorf("Gain.IsLowerThan() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGain_String(t *testing.T) {
	tests := []struct {
		name     string
		gain     Gain
		expected string
	}{
		{
			name:     "正数",
			gain:     Gain{HardScore: 1, SoftScore: 2.0},
			expected: "{1, 2.000000}",
		},
		{
			name:     "负数",
			gain:     Gain{HardScore: -1, SoftScore: -2.0},
			expected: "{-1, -2.000000}",
		},
		{
			name:     "零值",
			gain:     Gain{HardScore: 0, SoftScore: 0.0},
			expected: "{0, 0.000000}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.gain.String(); got != tt.expected {
				t.Errorf("Gain.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}
