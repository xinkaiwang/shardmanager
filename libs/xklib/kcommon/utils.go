package kcommon

import (
	"os"
	"strconv"
)

// getEnvInt 从环境变量获取整数值，如果不存在或无效则返回默认值
func GetEnvInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvString 从环境变量获取整数值，如果不存则返回默认值
func GetEnvString(key string, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func BoolToInt8(b bool) int8 {
	if b {
		return 1
	}
	return 0
}

func BoolFromInt8(v int8) bool {
	return v != 0
}
