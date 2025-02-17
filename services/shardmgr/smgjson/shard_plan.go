package smgjson

import "strings"

// ShardPlan is a file, each line in this file is a shard line.
// etcd path is /smg/config/shard_plan.txt
type ShardPlan struct {
	// ShardLines 是 shard 的列表
	ShardLines []*ShardLine
}

func ParseShardPlan(plan string, defVal ShardHints) *ShardPlan {
	// 按行分割
	lines := strings.Split(plan, "\n")

	// 创建 ShardPlan
	sp := &ShardPlan{
		ShardLines: make([]*ShardLine, 0, len(lines)),
	}

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
		sp.ShardLines = append(sp.ShardLines, sl.ToShardLine(defVal))
	}

	return sp
}
