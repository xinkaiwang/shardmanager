package smgjson

// Bool2Int8 将 bool 值转换为 int8，用于 JSON 序列化
// true -> 1, false -> 0
func Bool2Int8(b bool) int8 {
	if b {
		return 1
	}
	return 0
}

// Int82Bool 将 int8 值转换为 bool，用于 JSON 反序列化
// 非零值 -> true, 0 -> false
func Int82Bool(i int8) bool {
	return i != 0
}
