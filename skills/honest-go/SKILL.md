---
name: honest-go
description: Honest Go — the Go code style used across xinkai's projects: names describe semantics not implementation, callers carry zero burden, searchability is first-class. Covers constructor and type naming, interface sizing, hiding implementation behind accessors, the panic-with-typed-error discipline and when to return instead, the GetCurrentXxx singleton pattern, and comment style. Version-free by design — it contains no library API snippets. Invoke when writing or reviewing Go in a project that follows Honest Go, when naming a new type/constructor/interface, when deciding whether a function should panic or return an error, or when the user says "honest go" / "is this honest go" / "review naming".
---

# Honest Go

**Honest Go = code that is explicit, searchable, and hides implementation
details from callers.**

Three principles everything else follows from:

1. **Names describe semantics, not implementation** — `NewDiskStorage()`, not `New()`
2. **Callers carry zero burden** — never expose raw bytes, wire formats, or internal types
3. **Searchability is first-class** — cmd-click on any symbol lands somewhere useful

## What this file deliberately does not contain

No library API examples. Honest Go is *style*; the projects that follow it also
use xklib, and how to call xklib changes when xklib releases. Those instructions
ship with the library:

```bash
go list -m -f '{{.Dir}}' github.com/xinkaiwang/shardmanager/libs/xklib
# → AGENTS.md (rules) and UPGRADING.md (version migrations)
```

This separation is not tidiness. The previous combined style doc taught
`klogging.InitOpenTelemetry()` for months after that function was deleted, and
misspelled two other API names, because version-bound content had been copied
into a version-free document. **When a rule would change if a library releases a
new version, it does not belong here.**

## Naming

- Constructors always include the type: `NewDiskStorage()`, `NewS3Storage()` —
  never bare `New()`.
- No `Manager` / `Handler` / `Helper` suffixes. Name by what it does.
- Semantics over role: name by what a thing *means*, not by its HTTP role or its
  wire-format field name.
- Pairs should be obvious: `DiskStorage` ↔ `S3Storage`, `Get` ↔ `Put`.
- Short names are fine for interfaces (`Storage`), not for constructors.
- Unexported fields, exported methods.

The searchability test: if a reader greps the symbol, do they land on the code
that matters? A helper that dispatches on a string discriminator to N distinct
targets fails this — `grep <target>` stops finding the call sites that affect it.

## Interfaces

- Small: 1–3 methods.
- YAGNI: no interface until the second implementation exists.
- No unnecessary middle layers.

## Hiding implementation

- Private fields plus typed accessors. Never expose `[]byte` as a public field.
- No anonymous struct fields.
- Split structs that share a name but not fields.

## Errors: panic by default, return by exception

**Default — panic with a typed, structured error.** Business and infrastructure
failures travel up on their own; the call sites stay clean; the HTTP/RPC boundary
recovers once and converts to a response.

**Return an error instead when** the failure is an expected outcome rather than a
broken invariant — an upstream provider reporting overload, a lookup that
legitimately misses — or when the caller, not the boundary, must decide the retry.

**Never:**
- `panic("some string")` — a raw value carries no type, no fields, no cause.
- Returning a bare `err` from a lower layer with no context added.
- Checking an error by matching on its message text. Assert the type and compare
  the error's declared type field.

Error type names are PascalCase and say what failed: `TenantNotFound`,
`OpenMySQLFailed`. Structured fields go in the error as key/value pairs, never
concatenated into the message — a concatenated ID cannot be indexed or filtered.

The concrete API for creating, wrapping, and inspecting these errors is in the
library's shipped `AGENTS.md`.

## Singletons: the `GetCurrentXxx` pattern

```go
var currentThing Thing

func SetCurrentThing(t Thing) { currentThing = t }

func GetCurrentThing(ctx context.Context) Thing {
    if currentThing == nil {
        currentThing = NewDefaultThing(ctx)   // lazy default
    }
    return currentThing
}
```

Shape: package-level variable, `SetCurrent*` / `GetCurrent*` pair, lazy default
inside the getter. The point is testability — a test swaps the implementation
without a DI framework, and production code never names a concrete type.

Related: an IO-layer dependency should sit behind an interface with
subject-named implementations (`RedisXxx` / `MemoryXxx` / `DiskXxx`), reached
through this pattern. A struct that wraps a concrete client directly couples its
package to that medium forever.

## Comments

- Package comment on every package: `// Package foo does X.`
- Comment the **why**, never the **what**. If the reader can see it from the
  code, deleting the comment improves the file.
- One line is almost always enough.
- For types that cross a boundary, show direction and a real example value.

## Bootstrapping a new project

Nothing to install. This skill and `using-xklib` are symlinked into every
runtime's global directory, so they are already present in any repo on this
machine. Three steps, all inside the new project:

**1. Declare it** — in the project's `CLAUDE.md` (or `AGENTS.md`; use whichever
the agents you run there read). An agent cannot guess that a project follows a
style:

```markdown
## Conventions

This project follows Honest Go (skill `honest-go`) and depends on xklib.
Library rules ship with the module:
`go list -m -f '{{.Dir}}' github.com/xinkaiwang/shardmanager/libs/xklib`
→ AGENTS.md (authoring rules), UPGRADING.md (version migrations).

Three contracts, violated by default because ordinary Go idiom is wrong here,
and breaking them produces no error and no visible symptom:

1. Log with `slog.XxxContext(ctx, ...)`, never `slog.Xxx(...)`.
2. Every log line carries `slog.String("event", "...")`. Casing: <pick one and
   say so — CamelCase or snake_case — then never mix>.
3. Never call `time.Now/Sleep/After/AfterFunc`; use `kcommon.GetWallTimeMs()`,
   `GetMonoTimeMs()`, `SleepMs(ctx, ms)`, `ScheduleRun(ms, fn)`.

Errors: panic with a typed kerror; the HTTP/RPC boundary recovers once.
```

Only those three are inlined. Everything else is pulled on demand — style from
this skill, API from the module's shipped docs — because everything else is
visible in review and already has a `smell-*` scanner behind it.

**2. Depend on the library**

```bash
go get github.com/xinkaiwang/shardmanager/libs/xklib@latest
```

**3. Wire `main()`** — copy the startup block from the module's `README.md`
§ "Wiring it into main()" **whole**. Four of its six steps fail silently when
omitted (metrics recorded and scraped by nobody, process gauges reading a steady
zero, logs with no `trace_id`, requests starting their own trace). Do not trim
it to "the parts we need".

Then write code. The agent has the style here, the API in the module, and the
three contracts in front of it at all times.

## Related skills

The `smell-*` family scans for violations of these rules after code exists:

| Skill | Rule it enforces |
| --- | --- |
| S13 `grep-defeating-helper` | Searchability — principle 3 |
| S1 `bespoke-io-shape` | Interface + subject-named impl + `GetCurrentXxx` provider |
| S12 `type-erasure-at-error-boundary` | Typed errors surviving across boundaries |
| S5 `foundation-amnesia` | New code diverging from established conventions |

This skill is the authoring side: it states the rule, those state the detection.
