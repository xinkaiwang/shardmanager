---
name: using-xklib
description: Authoring rules for writing Go code in a project that depends on xklib (github.com/xinkaiwang/shardmanager/libs/xklib) — logging that carries trace IDs, the `event` field convention, the kcommon time abstraction that virtual-clock tests depend on, kerror typed errors, kmetrics declaration/registration, and the krunloop adopt-or-not decision. Unlike the scan-only smell-* family, this one fires BEFORE code is written. Invoke when the user says "wire up xklib" / "set up a new service with xklib" / "add a metric" / "should I use krunloop", or proactively whenever you are about to write a log line, a metric, an error return, or any time-dependent code in a repo whose go.mod requires xklib.
---

# Using xklib

**Authoring skill** — the `smell-*` family scans code after the fact; this one
tells you how to write it right the first time. Load it before generating code,
not during review.

## When to invoke

- Setting up a new service that uses xklib (`main()` wiring)
- About to write **any** `slog.*` call, metric declaration, error return, or
  time-dependent logic in a repo that depends on xklib
- "should this be a runloop?" / "how do I test this timeout without waiting?"
- The user asks how another project should consume xklib

Check whether it applies: `grep xklib go.mod`. If the module is not a
dependency, this skill does not apply.

## Step 1 — read the shipped instructions

The authoritative rules ship **inside the module**, so they match the exact
version this project resolved. Do not work from memory or from a snippet in
some other repo's docs — those drift.

```bash
d=$(go list -m -f '{{.Dir}}' github.com/xinkaiwang/shardmanager/libs/xklib) && cat "$d/AGENTS.md"
```

Fallbacks, in order:
1. `$d/README.md` — the human manual; same system, prose form. Use it if
   `AGENTS.md` is absent (module older than v0.2.x).
2. If you are working inside the shardmanager monorepo itself, read
   `libs/xklib/AGENTS.md` directly.
3. If neither resolves, say so rather than guessing an API. xklib's surface is
   small but non-obvious, and invented calls will not compile.

Then follow that file. It is the source of truth; everything below is only what
this skill adds on top.

## Step 2 — the part that must survive without reading anything

If you take nothing else from this skill, take these three. They are violated
**by default**, because ordinary Go idiom is the wrong answer:

1. `slog.XxxContext(ctx, ...)`, never `slog.Xxx(...)` — trace IDs live in the
   ctx; a non-ctx line is invisible to any trace search.
2. Every log line carries `slog.String("event", "SomeName")`.
3. Never `time.Now/Sleep/After/AfterFunc` — use `kcommon.GetWallTimeMs()`,
   `GetMonoTimeMs()`, `SleepMs(ctx, ms)`, `ScheduleRun(ms, fn)`.

These three belong in the consuming project's `CLAUDE.md` / `AGENTS.md` too —
always-on context, because an agent breaking them has no idea it should look
anything up. `AGENTS.md` § 6 has a paste-ready block; offer to add it if the
project has none.

## Step 3 — the judgment calls

Most of xklib is additive and mechanical. Two decisions are not, and defaulting
either way is a mistake:

**Adopting `krunloop`** changes the concurrency architecture: one goroutine owns
one piece of state, every mutation becomes an event, the queue is unbounded, and
a slow event blocks all others. Right when one coherent piece of state is
mutated from many directions; wrong for stateless request handling. Surface the
trade-off to the user instead of deciding silently.

**Wiring `main()`**: copy the README block whole. Four of its six steps fail
*silently* when omitted — metrics recorded and scraped by nobody, process gauges
reading a steady zero that looks like a healthy idle process, logs that look
correct but carry no `trace_id`. An agent trimming this block to "what we need"
produces a service that looks wired and is not.

## Iron rules

- Read the shipped `AGENTS.md` before generating code; do not reconstruct the
  API from memory.
- Never invent an xklib function. If you cannot find it in the module source,
  it does not exist — check the "do not hand-roll" table before writing a
  helper, and check the package source before writing a call.
- Do not silently trim the `main()` wiring block.
- When the concurrency model is in question, ask rather than default.
