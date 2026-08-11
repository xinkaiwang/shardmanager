# xklib — rules for AI coding agents

You are writing Go code in a project that depends on xklib. This file is the
authoritative instruction set for doing that correctly. It ships inside the
module, so it describes **the exact version you resolved** — not whatever was
current when someone last copied a snippet.

Humans: read [README.md](README.md) instead. It explains the same system as
prose. This file is rules and wrong/right pairs, which is what an agent needs
at the moment it writes a line.

Locate this file from a consuming project:

```
go list -m -f '{{.Dir}}' github.com/xinkaiwang/shardmanager/libs/xklib
```

Upgrading from an older xklib? Do [UPGRADING.md](UPGRADING.md) first — it ships
alongside this file and has one section per version jump, with discovery
commands and exact before/after edits.

---

## 1. The three contracts

These are violated by default, because ordinary Go idiom is the wrong answer
here. Check them on every line you write, not at review time.

### 1.1 Log with the `Context` variants — always

The handler reads trace IDs and ambient fields **out of the ctx** at log time.
A non-ctx call still prints; it just can never be joined to a trace. Nothing
warns you.

```go
// WRONG — line appears in the log, invisible to any trace_id search
slog.Error("failed to load shard", slog.Any("error", err))

// RIGHT
slog.ErrorContext(ctx, "failed to load shard", slog.Any("error", err))
```

If the function has no ctx, thread one in. If it genuinely cannot have one
(package init, a bare goroutine with no caller), that is the only case where
`context.Background()` is acceptable — and it deserves a comment saying why.

### 1.2 Every log line carries an `event` field

`event` is a stable machine name; the message is for humans. Operators grep by
`event`, and the log-volume metric is labelled by it — a line without one lands
in an empty-keyed series.

```go
// WRONG — message is the only handle, and it will be reworded someday
slog.InfoContext(ctx, "worker went offline")

// RIGHT
slog.InfoContext(ctx, "worker went offline",
    slog.String("event", "WorkerOffline"),
    slog.String("workerId", id))
```

Name it after what happened, not after where the code is.

Casing is **your project's convention, not xklib's** — shardmanager uses
`WorkerOffline`, other consumers use `worker_offline`. xklib requires the field
to exist (the handler labels log-volume metrics by it); it does not dictate the
style. Match the surrounding code, and follow your project's `CLAUDE.md` /
`AGENTS.md` if it states one.

### 1.3 Never call `time.Now`, `time.Sleep`, `time.After`, `time.AfterFunc`

Go through kcommon. This indirection exists so tests can replace the clock and
drive minutes of protocol time in milliseconds of real time.

```go
// WRONG — this line is a hole the virtual clock cannot patch
deadline := time.Now().UnixMilli() + 30_000
time.Sleep(5 * time.Second)
time.AfterFunc(2*time.Second, retry)

// RIGHT
deadline := kcommon.GetWallTimeMs() + 30_000
kcommon.SleepMs(ctx, 5_000)
kcommon.ScheduleRun(2_000, retry)
```

`GetWallTimeMs` for timestamps you compare against stored/wire values,
`GetMonoTimeMs` for measuring elapsed time. One direct `time.Now()` on a
production path does not fail a test — it makes the virtual-time tests flaky,
which is worse.

---

## 2. Do not hand-roll what already exists

Before writing a helper, check this table. Reinventing these is the most common
agent failure in this codebase.

| About to write | Use instead |
| --- | --- |
| A typed error with fields, or `fmt.Errorf("...%w")` | `kerror.Create(type, msg).With(k, v)` / `kerror.Wrap(err, ...)` |
| Retry/backoff jitter (`rand.Intn` around a base) | `kcommon.RandomlizeValueByRatio(ctx, base, 0.1)` |
| A random ID / request ID | `kcommon.RandomString(ctx, n)` / `kcommon.NewTraceId(ctx, prefix, n)` |
| `defer func(){ recover() }()` turning panic into error | `kcommon.TryCatchRun(ctx, fn)` (or `TryCatchRunWithStack`) |
| `os.Getenv` + `strconv.Atoi` + default | `kcommon.GetEnvInt(key, def)` / `GetEnvString` |
| `mu.Lock(); defer mu.Unlock()` around a small block | `kcommon.RunWithLock(&mu, func(){ ... })` |
| Timing a function and emitting latency metrics | `kmetrics.InstrumentSummaryRunVoid(ctx, name, fn, notes)` |
| HTTP middleware that starts a span | `klogging.TracingMiddleware(handler)` |
| Parsing a log level string | `klogging.ParseLevel(s)` |
| Goroutine collecting heap/goroutine/GC gauges | `ksysmetrics.StartSysMetricsCollector(...)` |

---

## 3. Recipes

### 3.1 Wiring a new service

Copy the startup block from [README.md](README.md) § "Wiring it into main()"
**in full**. Every line switches on one capability and every omission fails
silently — the README's table lists exactly what each failure looks like. Do
not trim it to "the parts we need"; the parts that look optional are the ones
whose absence is invisible.

### 3.2 Adding a metric

```go
// package level — CreateKmetric self-registers on construction
var ShardAssignMetric = kmetrics.CreateKmetric(context.Background(),
    "shard_assign_ms",                              // no _count/_sum suffix: those are generated
    "time to assign one shard (count = assignments, sum = ms)",
    []string{"service", "result"})                  // tag NAMES, order fixed

// call site — tag VALUES, same count and order, or it panics
ShardAssignMetric.GetTimeSequence(ctx, svc, "ok").Add(elapsedMs)
```

Rules:
- One `Kmetric` emits `<name>_count` and `<name>_sum`. Add `.CountOnly()` when
  a sum is meaningless (pure event counter).
- Write a description an operator can read. `"desc"` is not a description.
- If you add a `_drop` or `_error` counter, make sure a denominator exists —
  a numerator alone cannot answer "what fraction".
- A metric nobody scrapes is not done: the registry must be attached to a
  producer in `main()` (README step 4).

### 3.3 Returning an error

```go
// new error
ke := kerror.Create("ShardNotFound", "no such shard").
    With("shardId", id).                   // structured field, never concatenated into the message
    WithErrorCode(kerror.EC_RETRYABLE)     // only if a retry could actually succeed
panic(ke)

// wrapping a cause
panic(kerror.Wrap(err, "EtcdReadFailed", "cannot read shard plan", true /*needStack*/))
```

**Panic or return?** That is the consuming project's style, not xklib's, and
both callers of this library panic by default — shardmanager has 195 `panic(ke)`
sites and zero `return kerror`. Match the code around you; if the project
follows Honest Go, panic with the typed error and let the HTTP/RPC boundary
recover once, returning only when the failure is an expected outcome the caller
must branch on. Never `panic("some string")` — a raw value carries no type, no
fields, no cause.

- **An uncaught panic prints the outer layer only.** Go renders a panic value
  through `Error()`, and `Kerror.Error()` is `ShortString()` — type, message and
  detail fields, but **not the cause and not the stack**. So
  `panic(kerror.Wrap(err, "ListenFailed", ...))` that escapes to the top prints
  `ListenFailed: http listener stopped, addr=:8080` and swallows the
  `bind: address already in use` underneath it. Where the cause is the whole
  diagnosis, either log `ke.FullString()` at the recover boundary, or lift the
  cause into a detail field: `.With("cause", err.Error())`.
- `Create` captures a stack. On a hot path where you do not need it, chain
  `.WithoutStack()`.
- Test retryability with `kerror.Retryable(err)`, which works on a plain
  `error`. Check the kind by asserting `*kerror.Kerror` and comparing `.Type`
  — never by matching on the message text.
- Where a value is declared rather than panicked, declare the concrete type:
  `*kerror.Kerror`, not `error`. An `error` field or return forces every
  consumer to type-assert or string-match.

### 3.4 Testing code that depends on time

Two modes; pick by whether async work is involved.

**Pure computation, no goroutines** — advance the clock by assignment:

```go
fakeTime := kcommon.NewFakeTimeProvider(1_000_000)
kcommon.RunWithTimeProvider(fakeTime, func() {
    dt := NewDecayingThreshold()
    fakeTime.WallTime += 30_000            // 30 protocol-seconds later
    assert.Less(t, dt.GetCurrent(fakeTime.WallTime), initial)
})
```

**A live system with runloops/timers** — drive the simulation:

```go
kcommon.RunWithTimeProvider(fakeTime, func() {
    // ... start the system under test ...
    fakeTime.VirtualTimeForward(ctx, 30*1000)   // runs every task due in 30s
    assert.Equal(t, WS_Draining, state())
})
```

`VirtualTimeForward` runs due tasks in order, freezes the clock while events
are still being processed, and jumps to the next due task otherwise. It panics
(typed kerror, with the scene attached) if the simulation cannot reach the
target time — treat that panic as "the system under test deadlocked", not as a
flake to retry.

Do not call `kcommon.SetTimeProvider` from a running test — it writes a plain
global with no lock. Use `RunWithTimeProvider` to scope it.

---

## 4. A decision you must not make on autopilot: krunloop

`krunloop` is not a utility you sprinkle in. Adopting it means one goroutine
owns one piece of mutable state and every mutation becomes an event.

Adopt it when: one coherent piece of state is mutated from many directions
(watchers, timers, HTTP handlers) and you would otherwise be reasoning about
lock ordering.

Do **not** adopt it for stateless request handling — plain handlers are simpler
and run in parallel.

If you adopt it, you own these consequences (see README § krunloop): the queue
is unbounded, one slow event blocks all others, `StopAndWaitForExit` waits
indefinitely by design, and producers must be stopped before the loop that
consumes from them.

When unsure, ask the human rather than defaulting either way.

---

## 5. Before you claim the work is done

Silent failures, in the order they bite:

- [ ] Every new `slog.*` call is a `*Context` variant and has an `event` field.
- [ ] No `time.Now` / `time.Sleep` / `time.After*` outside tests.
- [ ] New metrics: description is real, tag counts match, denominator exists,
      registry is attached to a producer in `main()`.
- [ ] Ran the tests, including with `-count=1` (Go caches passes).
- [ ] Did not add a dependency to reproduce something in the table in § 2.

---

## 6. Paste this into your project's CLAUDE.md / AGENTS.md

The three contracts must be in always-on context, because an agent violating
them does not know it needs to look anything up. Keep it short — this is
competing for attention with everything else in the file.

```markdown
## xklib conventions (non-negotiable)

This project depends on xklib. Three rules, violated by default because normal
Go idiom is wrong here:

1. Log with `slog.XxxContext(ctx, ...)`, never `slog.Xxx(...)` — trace IDs come
   from the ctx; a non-ctx line can never be joined to a trace.
2. Every log line carries `slog.String("event", "SomeName")` — a stable machine
   name, separate from the human message.
3. Never call `time.Now/Sleep/After/AfterFunc` — use `kcommon.GetWallTimeMs()`,
   `GetMonoTimeMs()`, `SleepMs(ctx, ms)`, `ScheduleRun(ms, fn)`. Direct time
   calls silently break the virtual-clock tests.

Full agent instructions ship with the module:
`go list -m -f '{{.Dir}}' github.com/xinkaiwang/shardmanager/libs/xklib`/AGENTS.md
```
