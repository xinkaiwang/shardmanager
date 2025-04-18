package smgjson

import "github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"

// 指针辅助函数集合
// 这些函数用于简化创建指向特定类型的指针的过程

// NewShardStateJsonPointer 创建一个指向给定 ShardStateJson 的指针
func NewServiceTypePointer(st data.StatefulType) *data.StatefulType {
	return &st
}

// NewMovePolicyPointer 创建一个指向给定 MovePolicy 的指针
func NewMovePolicyPointer(v MovePolicy) *MovePolicy {
	return &v
}

// NewInt8Pointer 创建一个指向给定 int32 的指针
func NewInt8Pointer(v int8) *int8 {
	return &v
}

// NewInt32Pointer 创建一个指向给定 int32 的指针
func NewInt32Pointer(v int32) *int32 {
	return &v
}

// NewInt64Pointer 创建一个指向给定 int64 的指针
func NewInt64Pointer(v int64) *int64 {
	return &v
}

// NewStringPointer 创建一个指向给定 string 的指针
func NewStringPointer(v string) *string {
	return &v
}

// NewBoolPointer 创建一个指向给定 bool 的指针
func NewBoolPointer(v bool) *bool {
	return &v
}

// NewFloat64Pointer 创建一个指向给定 float64 的指针
func NewFloat64Pointer(v float64) *float64 {
	return &v
}
