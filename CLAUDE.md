# General Rules

1. 本文件包含的是针对本项目的特定规则和经验教训，优先级高于任何通用知识。
2. 总是用中文回答我，即使我有时会用英文提问。因为中文是我的母语，我更习惯用中文阅读，但是我的键盘输入英文比较快。

# 判断纪律（2026-08-09 CtxInfoRevisit 讨论沉淀，完整推导见 research/2026_0809.CtxInfoRevisit/notes.md）

3. **定性/定级必须给出具体 failure scenario**（谁、什么场景、什么损害）；写不出后果链的只能记"观察"，不得记"问题"。禁止凭"最佳实践"标签下判断——反面教材：我曾凭 "crypto rand=好/伪随机=坏" 误判 kcommon（trace ID 只需唯一性，不需不可预测性；真熵采集速率极低，headless 机器仅个位~百级 bits/s）。
4. **审计判断从"消费者需要什么性质"出发推导**，不从"用了什么技术"贴标签；量化判断算数量级（熵预算/QPS/临界区），算不出就明说是估计。
5. **xklib 是"一盒胶带和笔"**：原件简单、独立、可见全貌；组装发生在使用者的 diff 里；库背后不得有默认运行的复杂度（反例：默认注册的 Baggage propagator、log4j JNDI）。"功能多=好"是必须抵制的互联网先验，未定价的功能是负资产。不得以本仓库 grep 用量判断库功能取舍——使用者在仓库之外（此错一天犯过两次：Importance 分级、B3）。
6. **对开源依赖与对我自己的输出适用同一信任模型**：不信默认输出，只信过验货的结论（读源码给 file:line 收据、跑 demo 实证、算数量级）。依赖热路径审计优先级：隐藏串行点（channel/单 worker/全局锁）> 每操作分配。opencensus 的 QPS 封顶病根 = stats 全走单 channel + 单 worker goroutine（worker.go）。
7. **重要讨论落盘**：research/<date>.<topic>/notes.md，问题编号化（如 KLOG-NNN）+ 决策日志（含否决理由）。用户的质疑是流程的承重结构，被顶撞后修正结论是常态，不要护短。

# 编码约定

本项目遵循 **Honest Go**（skill `honest-go`），并且**自身就是 xklib 的所在地**——
`libs/xklib/AGENTS.md` 与 `UPGRADING.md` 是写给外部消费者的，改动它们时记住读者
不在本仓库内（第 5 条）。

三条违反后**无声**的契约，所以写在这里而不是等 skill 触发：

1. 日志一律用 `slog.XxxContext(ctx, ...)`，不用 `slog.Xxx(...)`——trace_id 从 ctx 里读，
   非 ctx 版本照样打印，只是永远无法与 trace 关联。
2. 每行日志带 `slog.String("event", "SomeName")`；本仓库用 CamelCase（消费方各随其项目约定）。
3. 永不直接调 `time.Now/Sleep/After/AfterFunc`，一律走 `kcommon.GetWallTimeMs()`、
   `GetMonoTimeMs()`、`SleepMs(ctx, ms)`、`ScheduleRun(ms, fn)`——生产路径上一处直接调用
   就是虚拟时钟补不上的洞，它让 fake-time 测试变 flaky 而不是变红。

错误处理：`panic(kerror.Create(...).With(...))`，由 HTTP/RPC 边界统一 recover；
本仓库 195 处 panic、0 处 return kerror，新代码保持一致。

# 术语表

- **IAI** (Internet Average Intelligence, 互联网平均智力)：未经验证的共识先验输出——标签化判断（"X=好/Y=坏"）、功能广度当价值、置信语气与推导深度脱钩、照抄行业标准配方。开源项目的默认设计和 AI 的默认输出都是 IAI 产地。用户说"this is IAI"= 指出我在背标签，应立即补后果链或认错；"IAI code"= 配方抄来、未为本项目定价的代码。第 3–6 条纪律即为反 IAI 而立。

