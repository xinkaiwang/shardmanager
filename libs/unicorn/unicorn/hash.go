package unicorn

import "unicode/utf16"

// JavaStringHashCode 计算 Java 字符串的 hash code.
// this is 100% same as Java's String.hashCode().
func JavaStringHashCode(s string) int32 {
	var h int32 = 0
	// 转成 Java 的 UTF-16 编码单元
	utf16s := utf16.Encode([]rune(s))
	for _, c := range utf16s {
		h = 31*h + int32(c)
	}
	return h
}
