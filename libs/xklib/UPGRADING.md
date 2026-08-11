# Upgrading xklib

Migration instructions, one section per version jump. They ship inside the
module, so they describe the version you are upgrading **to**.

**If you are an agent performing the upgrade:** do the steps in order. Each has
a discovery command, an exact before/after, and a check. Steps marked
**REQUIRED** are compile breaks — the build fails until they are done. Steps
marked **DECISION** need a human answer; stop and ask rather than guessing.

---

## v0.1.x → v0.2.x

Two compile breaks, one capability you probably never had, and three behavior
changes in `krunloop` that need no code edit but will change what you see.

### Step 1 — REQUIRED: `klogging.InitOpenTelemetry()` was removed

It configured trace **header formats** and nothing else. It is replaced by a
function with an honest name.

```bash
grep -rn "InitOpenTelemetry" --include="*.go" .
```

```go
// BEFORE
klogging.InitOpenTelemetry()

// AFTER
klogging.InitDefaultPropagator()
```

One behavior difference: the old call also registered the W3C **Baggage**
propagator; the new one deliberately does not. Baggage silently copies ctx
key/values onto every outgoing request header — a cross-service data channel
this project rejected on purpose.

**DECISION** — check first:

```bash
grep -rni "baggage" --include="*.go" .
```

No hits (the normal case): proceed, the removal is a no-op for you. Hits: stop
and ask the human. Something in your code depends on a channel that is now
closed, and silently losing it is worse than a compile error.

### Step 2 — REQUIRED (only if you use the virtual clock): `VirtualTimeForward` returns nothing

```bash
grep -rn "VirtualTimeForward" --include="*.go" .
```

```go
// BEFORE — the bool meant "reached the deadline"; nobody ever checked it
ok := fakeTime.VirtualTimeForward(ctx, 30_000)

// AFTER
fakeTime.VirtualTimeForward(ctx, 30_000)
```

The give-up paths now log and panic with a typed `*kerror.Kerror` carrying the
scene (`forwardMs`, `virtualTimeMs`, `inFlightWork`, `pendingTasks`). If that
panic fires, the system under test deadlocked — treat it as a real failure, not
a flake to retry.

### Step 3 — RECOMMENDED: you have never had `trace_id` in your logs

Not a break; a gap you probably did not know about. v0.1.x installed a
propagator but **no TracerProvider**, so the global tracer was OpenTelemetry's
no-op: spans were created, none were valid, and the handler's trace injection —
which only fires for a valid span context — never ran. Every log line went out
without `trace_id`.

Add the provider next to the propagator:

```go
klogging.InitDefaultPropagator()

tp := klogging.InitDefaultTracerProvider("my-service",
    klogging.ParseSampleRatio(os.Getenv("TRACE_SAMPLE_RATIO")))
defer klogging.ShutdownTracerProvider(ctx, tp)
```

No-export mode: real trace/span IDs and local sampling decisions, no exporter,
no background worker, nothing shipped anywhere. That is all log correlation
needs. `ParseSampleRatio("")` returns 0, which still generates IDs — only the
sampled-request debug boost stays off.

**DECISION** — do not add this blindly if the project already correlates logs
some other way. Two checks, because the second one is the easy miss:

```bash
# (a) an OpenTelemetry provider of your own — this call would override it
grep -rn "SetTracerProvider\|NewTracerProvider" --include="*.go" .

# (b) a hand-rolled correlation-ID system — greps clean for (a) yet collides
grep -rln "TraceID\|trace_id\|RequestID\|CorrelationID\|X-.*-Trace" --include="*.go" .
```

A project that mints its own request IDs, propagates them in a custom header,
and stores them in ctx will end up with **two** independent trace identities in
its logs — the home-grown one and OpenTelemetry's — with no relationship
between them. That is worse than having neither. Stop and ask the human whether
to adopt xklib's tracing and retire the home-grown one, keep the home-grown one
and skip this step, or bridge them.

Verify: start the service, hit an endpoint, confirm a log line carries
`trace_id`. If it does not, one of the two Init calls is missing or runs after
the span is created.

### Step 4 — Behavior changes in `krunloop`, no code edit needed

Skip if you do not import `krunloop`.

**`StopAndWaitForExit` now waits indefinitely.** It used to give up after 1000 ms
with a warning and return. It now waits for the in-flight event to finish,
logging a warning every second that names the stuck event. Returning early let
callers start tearing down state while a handler was still running on it — a
use-after-teardown that produced corruption instead of a hang. If your shutdown
now hangs, you have a genuinely stuck event handler; the warning names it.

**Enqueue after stop is now a loud drop.** It used to be an unguarded channel
send into a queue nobody was draining. Post-stop events now take a visible path:
a `WARN` naming the event plus a `runloop_enqueue_dropped_ct` metric. A sudden
appearance of these is a pre-existing teardown-order bug becoming visible — stop
producers (watchers, timers) before the loop that consumes from them — not a new
bug introduced by the upgrade.

**A timer leak is fixed for you.** Every `RunLoop` used to leave a self-
rescheduling 50 Hz sampler chain running forever after shutdown.
`StopAndWaitForExit` now terminates it. No action required.

### Step 5 — Verify

```bash
go get github.com/xinkaiwang/shardmanager/libs/xklib@v0.2.1
go mod tidy
go build ./...
go test ./... -count=1        # -count=1 or Go serves cached passes
grep -rn "InitOpenTelemetry" --include="*.go" .   # must be empty
```

### Optional: what else v0.2 gained

None of these require action; adopt them when you have the need.

| Addition | What it gives you |
| --- | --- |
| `klogging.TracingMiddleware(h)` | Per-request server span, upstream trace continued if present. |
| `klogging.CtxWithAttrs(ctx, ...)` | Ambient log fields: attach once, every log below that ctx carries them. `CtxWithAttrsLevel` gates by importance; `CtxWithProvider` attaches a live object read at log time. |
| `handler.SetLevel(lvl)` | Change log level at runtime — e.g. an admin endpoint turning on debug in prod. Affects handlers derived via `WithAttrs`/`WithGroup` too. |
| `klogging.Fatal(ctx, msg, ...)` | Log at FATAL and exit 1, with the line guaranteed flushed first. |
| `kcommon.InFlightWork*` | The quiescence counter the virtual clock waits on. You only touch this if you build your own event source. |

---

## Reporting a gap

If an upgrade step here was wrong or missing, that is a documentation bug worth
fixing at the source — the file lives at `libs/xklib/UPGRADING.md` in the
shardmanager repo and ships with every release.
