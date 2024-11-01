package kcommon

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTimer(t *testing.T) {
	ch := make(chan interface{})
	ScheduleRun(10, func() {
		ch <- nil
	})
	<-ch
	assert.Equal(t, true, true)
}

func TestMockTimer(t *testing.T) {
	mockTime := NewMockTimeProvider().SetAsDefault()
	ch := make(chan interface{}, 1)
	ScheduleRun(100, func() {
		ch <- nil
	})
	task := <-mockTime.ChTask
	task.Cb()
	<-ch
	assert.Equal(t, true, true)
}
