package kcommon

// HelloWithPanic 总是抛出一个非错误类型的 panic
func HelloWithPanic() {
	panic("this is a non-error panic")
}
