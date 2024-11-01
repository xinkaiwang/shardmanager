package kcommon

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFakeTimeProvider(t *testing.T) {
	time := NewFakeTimeProvider()

	RunWithTimeProvider(time, func() {
		res := 0
		ScheduleRun(100, func() {
			res = 100
		})
		ScheduleRun(10, func() {
			res = 10
		})

		assert.Equal(t, 0, res)
		time.SimulateForward(11)
		assert.Equal(t, 10, res)
		time.SimulateForward(101)
		assert.Equal(t, 100, res)

		ScheduleRun(1000000, func() {
			res = 1000000
		})
		time.SimulateForward(10000001)
		assert.Equal(t, 1000000, res)
	})
}
