package kcommon

import (
	"context"
	"time"
)

var (
	currentTimeProvider TimeProvider = NewSystemTimeProvider()
)

func RunWithTimeProvider(tp TimeProvider, fn func()) {
	old := currentTimeProvider
	currentTimeProvider = tp
	defer func() {
		currentTimeProvider = old
	}()

	fn()
}

func SetTimeProvider(provider TimeProvider) {
	currentTimeProvider = provider
}

type TimeProvider interface {
	GetWallTimeMs() int64
	GetMonoTimeMs() int64
	ScheduleRun(delayMs int, fn func())
	SleepMs(ctx context.Context, ms int)
}

func GetWallTimeMs() int64 {
	return currentTimeProvider.GetWallTimeMs()
}

func GetMonoTimeMs() int64 {
	return currentTimeProvider.GetMonoTimeMs()
}

func ScheduleRun(delayMs int, fn func()) {
	currentTimeProvider.ScheduleRun(delayMs, fn)
}

func SleepMs(ctx context.Context, ms int) {
	currentTimeProvider.SleepMs(ctx, ms)
}

// SystemTimeProvider: implements TimeProvider interface
type SystemTimeProvider struct {
	startTime time.Time
}

func NewSystemTimeProvider() *SystemTimeProvider {
	return &SystemTimeProvider{
		startTime: time.Now(),
	}
}

func (provider *SystemTimeProvider) GetWallTimeMs() int64 {
	now := time.Now()
	ms := int64(time.Nanosecond) * now.UnixNano() / int64(time.Millisecond)
	return ms
}

func (provider *SystemTimeProvider) GetMonoTimeMs() int64 {
	return time.Since(provider.startTime).Milliseconds()
}

func (provider *SystemTimeProvider) ScheduleRun(delayMs int, fn func()) {
	time.AfterFunc(time.Duration(delayMs)*time.Millisecond, fn)
}

func (provider *SystemTimeProvider) SleepMs(ctx context.Context, ms int) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

func (provider *SystemTimeProvider) SetAsDefault() *SystemTimeProvider {
	currentTimeProvider = provider
	return provider
}

type TimerTask struct {
	Cb      func()
	DelayMs int
}

// MockTimeProvider: implements TimeProvider interface
type MockTimeProvider struct {
	WallTime int64
	MonoTime int64

	ChTask chan *TimerTask
}

func NewMockTimeProvider() *MockTimeProvider {
	return &MockTimeProvider{
		ChTask: make(chan *TimerTask, 10),
	}
}

func (provider *MockTimeProvider) GetWallTimeMs() int64 {
	return provider.WallTime
}

func (provider *MockTimeProvider) GetMonoTimeMs() int64 {
	return provider.MonoTime
}

func (provider *MockTimeProvider) ScheduleRun(delayMs int, fn func()) {
	task := &TimerTask{
		Cb:      fn,
		DelayMs: delayMs,
	}
	provider.ChTask <- task
}

func (provider *MockTimeProvider) SetTimeMs(timeMs int64) *MockTimeProvider {
	provider.MonoTime = timeMs
	provider.WallTime = timeMs
	return provider
}

func (provider *MockTimeProvider) AddTimeMs(diffMs int64) *MockTimeProvider {
	provider.MonoTime += diffMs
	provider.WallTime += diffMs
	return provider
}

func (provider *MockTimeProvider) SleepMs(ctx context.Context, ms int) {
	provider.MonoTime += int64(ms)
	provider.WallTime += int64(ms)
}

func (provider *MockTimeProvider) SetAsDefault() *MockTimeProvider {
	currentTimeProvider = provider
	return provider
}
