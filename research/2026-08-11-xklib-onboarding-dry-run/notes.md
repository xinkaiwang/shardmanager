# xklib onboarding 实战验货（kitten 空仓库）

日期：2026-08-11
方法：在一个只有 CLAUDE.md 的空仓库里说一句 "onboard honest-go"，让 agent 自行
完成三步清单，然后**跑起来**验收，而不是读一遍觉得对。

结论先行：**文档全对、清单全对、生成的代码看着也全对，唯独不能编译。**
这轮验货的价值全部来自"跑"这个动作。

---

## OBD-001 · bootstrap 产出的仓库编译不过 【P1 · 已修（清单）】

两个独立原因：

1. **缺依赖**：装配块用 `contrib.go.opencensus.io/exporter/prometheus`，而清单
   只写了 `go get xklib`。xklib 故意不捆绑 exporter（选哪个是服务的决定），但
   清单没把这个"故意"翻译成第二条 `go get`。
2. **tidy 时序**：`go mod tidy` 在 `main.go` 存在之前跑过，于是 xklib 被记成
   `// indirect` 且缺 go.sum 条目；之后没人再 tidy。

**Failure scenario**：新项目第一天就是红的，而红的原因和"约定"无关——人会怀疑
是这套约定太复杂，实际只是清单少了两行。

**修法**：清单补第二个 `go get`；新增 **step 4 = build + vet + 真跑起来 curl**。
理由直接写进清单："Everything read correctly. Nothing compiled."

## OBD-002 · ksysmetrics 冷启动零窗口 【P2 · 已修（代码）】

`StartSysMetricsCollector` 只在 ticker 触发时采集，进程启动后的第一个 interval
（默认 15s）内所有进程 gauge 读 0。实测：T+2s → `kitten_process_goroutines 0`；
T+20s → `7`。

**Failure scenario**：每次 pod 重启，仪表盘上出现一段零平台；而在告警侧，它与
"根本没调 StartSysMetricsCollector"**完全同形**——正是 README 里记载的那个无声
失败模式。两种状态在窗口期不可区分。

**修法**：进 ticker 循环前先同步采一次。测试用一小时 interval 钉死——任何非零值
都只可能来自启动那次采集。

**这是本轮唯一的真库缺陷**，其余都是文档/流程缺口。它存在多久没人发现，因为
只有"启动后立刻 scrape"才看得见。

## OBD-003 · 未捕获 panic 丢掉 kerror 的 cause 【P3 · 已记录】

实测输出：

```
panic: ListenFailed: http listener stopped, listener=service, addr=:18080
```

真原因 `bind: address already in use` 一个字都没有。链路：Go 用 `Error()` 渲染
panic 值 → `Kerror.Error()` = `ShortString()` → `ToFullString(withStack=false,
withCause=false)`。**包进去的 cause 在默认 panic 输出里不可见。**

**Failure scenario**：进程因端口占用/权限/磁盘满而死，运维只看到"http listener
stopped"，得自己猜是哪一类。包装看起来做了工作，实际信息在打印时被丢掉。

**修法（未改库，改文档）**：AGENTS.md 记明这个陷阱，给两条出路——recover 边界打
`FullString()`，或把 cause 抬进 detail 字段 `.With("cause", err.Error())`。
不改 `Error()` 的语义：短形式在日志里是对的，问题出在 panic 这条路径上。

## OBD-004 · smoke 服务被命名为项目名 【P3 · 已修（清单）】

生成的是 `cmd/kitten/main.go`——一个 health-check 桩占用了真正的 kitten 二进制
将来要用的名字，且对下一个读者谎称这个目录里是产品。

**修法**：清单写明 smoke 服务叫 `hellosvc`（本仓库 `services/hellosvc` 已是先例），
真入口出现后删掉或改名。原则 1 对二进制同样适用。

## OBD-005 · 缺 Makefile 【P3 · 已修（约定）】

8 个模块 Makefile 100% 存在，但从没写下来，所以 bootstrap 不知道要生成。
target 频次：`test` 8/8、`clean` 8/8、`all` 8/8，`lint`/`fmt`/`help`/`build`/
`docker-build`/`docker-push` 各 7、`run` 6。

顺带settle 两处漂移：**`lint`**（7/9）而非 `vet`、**`docker-build`**（7/9）而非
`docker`。理由不是投票本身，而是"要猜的 target 名等于没有 target"。

## OBD-006 · 触发链路成立 【正面结果，记录在案】

skill 确实自动加载了。指纹是 CLAUDE.md 里那个**必填空被填上**：
`Casing: **CamelCase** (WorkerOffline, ShardAssigned) — never mix in snake_case`。
模板故意留空强迫作者当场定，这个空只可能由读过 skill 正文的 agent 填。

生成的代码也不是照抄，而是按 Honest Go 改写：`panic(kerror.Wrap(...))` 而非
`return err`；`kcommon.GetEnvInt` 而非手搓 `os.Getenv`+`Atoi`（"不要手搓"那张表
生效）；`cmd/<binary>/` 文件夹名 = 二进制名。

---

## 决策日志

| # | 决策 | 理由 |
|---|---|---|
| D1 | 清单加 step 4（build/vet/run/curl），不只是"写完就算" | 本轮全部三个真问题都只有"跑"才暴露；读文档零命中 |
| D2 | ksysmetrics 改代码，不是在文档里记"头 15s 是 0" | 它与无声失败同形，文档修不掉同形性 |
| D3 | `Error()` 语义不动，只记陷阱 | 短形式在日志路径上是对的；病在 panic 路径，改 `Error()` 会让每条日志变长 |
| D4 | Makefile target 名按出现频次定，不按个人偏好 | 需要猜的 target 名等于没有 |

## 遗留

- **xklib 需要 v0.2.2 tag**：kitten 锁在 v0.2.1，仍带 OBD-002 的零窗口缺陷。
- 一次验货只覆盖了"新建 Go 服务"这条路径。库模块（无 main、无 docker）的
  bootstrap 没验过。
