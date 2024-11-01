package kcommon

import "runtime"

var (
	mem = &runtime.MemStats{}
)

// GetHeapAlloc: bytes of allocated heap objects
func GetHeapAlloc() uint64 {
	runtime.ReadMemStats(mem)
	return mem.HeapAlloc
}

// GetIdleToAllocRatio: heap idle (unused) / allocation ratio
func GetIdleToAllocRatio() float64 {
	runtime.ReadMemStats(mem)
	return float64(mem.HeapIdle) / float64(mem.HeapAlloc)
}

// GetHeapIdle: heap idle (unused)
func GetHeapIdle() uint64 {
	runtime.ReadMemStats(mem)
	return mem.HeapIdle
}
