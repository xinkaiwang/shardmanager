package smgjson

import "strings"

// ParseShardPlan: ShardLine 表示一个 shard 的配置.
// list of ShardLine is a ShardPlan.
func ParseShardPlan(plan string) []*ShardLineJson {
	// 按行分割
	lines := strings.Split(plan, "\n")

	var list []*ShardLineJson
	// 逐行解析
	for _, line := range lines {
		// 去除前后空白
		line = strings.TrimSpace(line)

		// 跳过空行和注释行
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}

		// 如果行中有注释，只取注释前的部分
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
		}

		// 解析 ShardLine
		sl := ParseShardLine(line)

		// 添加到 ShardPlan
		list = append(list, sl)
	}

	return list
}
