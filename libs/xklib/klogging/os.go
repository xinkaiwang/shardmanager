package klogging

import "os"

var (
	currentOsProvider = NewSystemOsProvider()
)

type OsProvider interface {
	Exit(code int)
}

func OsExit(code int) {
	currentOsProvider.Exit(code)
}

type SystemOsProvider struct {
}

func NewSystemOsProvider() OsProvider {
	return &SystemOsProvider{}
}

func (provider *SystemOsProvider) Exit(code int) {
	os.Exit(code)
}

type MockOsProvider struct {
	ExitCb func(code int)
}

func NewMockOsProvider() *MockOsProvider {
	return &MockOsProvider{}
}

func (provider *MockOsProvider) SetAsDefault() *MockOsProvider {
	currentOsProvider = provider
	return provider
}

func (provider *MockOsProvider) Exit(code int) {
	provider.ExitCb(code)
}
