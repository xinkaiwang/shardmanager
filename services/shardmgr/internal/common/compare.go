package common

func MapEqual(m1, m2 map[string]string) bool {
	// 如果两个都是 nil，则相等
	if m1 == nil && m2 == nil {
		return true
	}
	// 如果只有一个是 nil，则不相等
	if m1 == nil || m2 == nil {
		return false
	}
	// 比较长度
	if len(m1) != len(m2) {
		return false
	}
	// 比较内容
	for key, val1 := range m1 {
		if val2, ok := m2[key]; !ok || val1 != val2 {
			return false
		}
	}
	return true
}
