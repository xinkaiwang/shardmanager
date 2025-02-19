package common

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
