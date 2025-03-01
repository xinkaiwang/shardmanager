package common

func MapEqual(m1, m2 map[string]string) bool {
	if len(m1) != len(m2) {
		return false
	}
	for key, val1 := range m1 {
		if val2, ok := m2[key]; !ok || val1 != val2 {
			return false
		}
	}
	return true
}
