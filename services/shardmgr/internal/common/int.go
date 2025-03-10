package common

func BoolFromInt8(i int8) bool {
	return i != 0
}

func Int8FromBool(b bool) int8 {
	if b {
		return 1
	}
	return 0
}
