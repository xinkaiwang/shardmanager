# xklib

Small, independent primitives for Go backend services: structured logging with
trace correlation, typed errors, metrics, an actor-style event loop, and a
virtual clock for tests.

**Design constraint:** every part is meant to be readable in one sitting and
usable on its own. Nothing runs by default — no background goroutine, no
registered propagator, no exporter — unless your `main()` asks for it. Assembly
happens in your code, visible in your diff.

This document covers what the source and [pkg.go.dev](https://pkg.go.dev/github.com/xinkaiwang/shardmanager/libs/xklib)
**cannot** tell you: how the pieces are wired together, which contracts you have
to honor for them to work, and what fails silently if you skip a step. For API
details, read the godoc or the source — both are current.

Writing code with an AI agent? [AGENTS.md](AGENTS.md) covers the same ground as
rules and wrong/right pairs, and ships inside the module so it matches whichever
version you resolved.

## Install

```
go get github.com/xinkaiwang/shardmanager/libs/xklib@v0.2.0
```

xklib is a Go module inside a monorepo, so its tags carry the subdirectory
prefix (`libs/xklib/v0.2.0`); the `@v0.2.0` query above resolves to it. Requires
Go ≥ 1.21.

What you inherit by importing: `klogging` pulls in OpenTelemetry (+ the B3
propagator), `kmetrics` and `ksysmetrics` pull in OpenCensus. `kerror`,
`kcommon` and `krunloop` add nothing beyond those.

## Wiring it into main()

Nothing below is optional-but-recommended — each line switches on one capability,
and skipping it fails **silently**, which is why this section exists.

```go
func main() {
    ctx := context.Background()

    // 1. Wire formats for incoming/outgoing trace headers.
    klogging.InitDefaultPropagator()

    // 2. Real span engine, no-export mode: genuine trace_id/span_id and local
    //    sampling decisions, nothing shipped anywhere.
    tp := klogging.InitDefaultTracerProvider("my-service",
        klogging.ParseSampleRatio(os.Getenv("TRACE_SAMPLE_RATIO")))
    defer klogging.ShutdownTracerProvider(ctx, tp)

    // 3. The slog handler. Install it as the default so every package's
    //    slog.XxxContext call goes through it.
    slog.SetDefault(slog.New(klogging.NewHandler(&klogging.HandlerOptions{
        Level:  klogging.ParseLevel(os.Getenv("LOG_LEVEL")),
        Format: "json",
    })))

    // 4. Export path: your metrics exist but reach nobody until their
    //    registries are attached to a producer manager an exporter reads.
    //    (prometheus = contrib.go.opencensus.io/exporter/prometheus,
    //     metricproducer = go.opencensus.io/metric/metricproducer)
    pe, err := prometheus.NewExporter(prometheus.Options{Namespace: "my_service"})
    if err != nil { log.Fatal(err) }
    metricproducer.GlobalManager().AddProducer(kmetrics.GetKmetricsRegistry())
    metricproducer.GlobalManager().AddProducer(ksysmetrics.GetRegistry())

    // 5. Process-level gauges (heap, goroutines, fds, GC). Call at most once.
    ksysmetrics.StartSysMetricsCollector(ctx, 15*time.Second, version)

    // 6. Per-request spans. Wrap whatever mux needs tracing.
    srv := &http.Server{Addr: ":8080", Handler: klogging.TracingMiddleware(mux)}
    // ... plus a separate listener serving `pe` on /metrics
}
```

What breaks if you skip each step:

| Skipped | Symptom |
| --- | --- |
| 1 `InitDefaultPropagator` | Upstream `traceparent` / `b3` headers are ignored; every request starts a fresh trace, so cross-service traces never join up. |
| 2 `InitDefaultTracerProvider` | The global tracer stays a no-op. Spans are created but invalid, so **`trace_id` never appears in any log line** and sampled-level logging never triggers. |
| 3 `slog.SetDefault(NewHandler(...))` | Logs still print via Go's default handler, minus trace injection, minus ambient ctx fields, minus log metrics. Easy to miss — output looks fine. |
| 4 `AddProducer(...)` | Metrics are recorded correctly and scraped by nobody. `/metrics` simply has no series of yours. This is the most common wiring mistake. |
| 5 `StartSysMetricsCollector` | The process gauges are registered (package `init()`) but nothing ever fills them — they scrape as a steady **0**, which reads like a healthy process with no goroutines. |
| 6 `TracingMiddleware` | Inbound requests get no server span; handler logs have no `trace_id`. |

Order matters for 1–3 only in that they must precede any span creation or log
you care about. 4–6 are independent.

## The packages

| Package | Reach for it when |
| --- | --- |
| `klogging` | You want structured logs that carry `trace_id`, plus ambient per-request fields and a runtime-adjustable level. Has its own [README](klogging/README.md) — the deep one. |
| `kerror` | You want typed errors (`Type` + ordered detail pairs + optional stack + cause chain) instead of `fmt.Errorf` strings. Works with `errors.Is/As`. |
| `kmetrics` | You want counters/summaries/histograms exported through OpenCensus. Declare once at package level, increment from anywhere. |
| `ksysmetrics` | You want the standard process gauges (heap, stack, RSS, goroutines, fds, GC) without writing the collector. |
| `krunloop` | You have shared mutable state and want a single-writer actor loop instead of locks. See below — this one changes your architecture. |
| `kcommon` | Time abstraction (the load-bearing one, see contracts), seeded PRNG, panic-to-kerror helpers, env-var readers. |

## Contracts you have to honor

These are cross-cutting: honoring them in your code is what makes the library
work. Breaking them fails quietly.

**Always log with the `Context` variants.** `slog.InfoContext(ctx, ...)`, never
`slog.Info(...)`. Trace IDs and ambient fields are read from the ctx at log
time; the non-ctx call silently produces a line that cannot be joined to a
trace.

**Give every log line an `event` field.** `slog.String("event", "OrderCreated")`
— a stable machine-readable name, distinct from the human message. The handler
extracts `event` to label log-volume metrics, and operators grep by it.

**Never call `time.Now()`, `time.Sleep()` or `time.AfterFunc()` directly.** Go
through `kcommon.GetWallTimeMs()`, `GetMonoTimeMs()`, `SleepMs()`,
`ScheduleRun()`. The entire payoff of that indirection is in tests: swapping in
`FakeTimeProvider` lets a test drive protocol time (lease timeouts, retry
backoff, heartbeat periods) in milliseconds of real time, deterministically. One
direct `time.Now()` in your production path is a hole the virtual clock cannot
patch, and it turns those tests flaky rather than failing them.

**Errors:** `kerror.Create("SomeType", "message").With("key", val)` for new ones,
`kerror.Wrap(err, ...)` to keep a cause chain. `Create` captures a stack — call
`.WithoutStack()` on hot paths where you do not need it. Mark retryability with
`.WithErrorCode(kerror.EC_RETRYABLE)`; callers test it via `kerror.Retryable(err)`
on a plain `error`.

**Metric shape:** one `Kmetric` emits `<name>_count` and `<name>_sum` series.
`.CountOnly()` suppresses the `_sum` half. Tag values passed to
`GetTimeSequence` must match the declared tag names in count and order — a
mismatch panics.

## Footguns

- **`kmetrics.CreateKmetric` self-registers on construction**, into a
  process-global singleton registry. The idiom is a package-level `var`; merely
  importing the package makes the metric exist. There is no unregister.
- **Tag-name collisions call `os.Exit(1)`.** If a metric's tag name collides
  with a registry global tag, `kmetrics` exits the process rather than returning
  an error — deliberate fail-fast, but it will surprise you at startup.
- **`klogging.Fatal` terminates the process** (exit 1, no deferred functions
  run, no span flush). It is for broken invariants, not for handled errors.
- **`kcommon.SetTimeProvider` writes a plain global with no lock.** Call it
  once during startup or test setup, never concurrently with running code. Use
  `RunWithTimeProvider(tp, fn)` to scope it in a test.
- **`kcommon.RandomString` / `RandomInt` are not cryptographically secure.**
  Crypto-seeded PRNG, deterministic thereafter: fine for IDs and jitter, never
  for tokens or anything an attacker must not predict.
- **`StartSysMetricsCollector` is not idempotent.** It registers two CPU gauges
  each time it runs, and OpenCensus silently orphans the earlier registration
  rather than erroring. Call it once.
- **Dead entropy fails loudly at startup.** `InitDefaultTracerProvider` forces
  the PRNG to seed early on purpose, so a broken entropy source panics at boot
  instead of on some request hours later.

## krunloop: read this before adopting it

The other packages are additive — you can use `kerror` alone and change nothing
else. `krunloop` is not: it is an architecture choice.

The model is single-writer. You hand one resource (your mutable state) to a
`RunLoop`, and every mutation arrives as an event that the loop processes one at
a time, in order, on one goroutine. Nothing else touches the resource, so the
resource itself needs no locks and no `sync.Map` — the loop *is* the mutual
exclusion. Read-only access from outside goes through `VisitResource` /
`VisitResourceAndWait`, which are just events like everything else.

What that buys you: no lock ordering to reason about, no partially-applied state
transitions, and a natural place to instrument (every event is timed, counted,
and given its own root trace, so `trace_id` groups all logs of one event).

What it costs you, stated plainly:

- **The queue is unbounded.** `PostEvent` never blocks and never drops; if
  producers outrun the loop, memory grows. Backlog is observable as
  `runloop_enqueue_ct_count - runloop_elapsed_ms_count`. Watch it.
- **A slow event blocks all others.** No parallelism inside one loop by design.
  Long work belongs in a separate goroutine that posts a completion event back.
- **`StopAndWaitForExit` waits indefinitely** for the in-flight event, logging a
  warning with the stuck event's name every second. It will not return early
  and let you tear down state underneath a running handler.
- **Teardown order is yours to get right.** Stop producers (watchers, timers)
  before the loop that consumes from them; a post-stop `PostEvent` is dropped
  loudly (warn + `runloop_enqueue_dropped_ct`), not silently.

Use it when one coherent piece of state is mutated from many directions. Do not
use it for stateless request handling — plain handlers are simpler and parallel.

## Where to look next

- **API reference:** godoc / pkg.go.dev, or the source — packages are small.
- **`klogging/README.md`:** the one package with its own deep documentation
  (handler options, level semantics, sampling, ambient ctx fields).
- **`research/`** in the repo root: numbered decision trails for the non-obvious
  choices — why Baggage propagation is deliberately absent, why the virtual
  clock waits on an in-flight counter, why the metrics layer avoids a single
  serialization point. Read these before "fixing" something that looks odd.
