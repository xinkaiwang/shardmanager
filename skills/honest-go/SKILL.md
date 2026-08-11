---
name: honest-go
description: Honest Go — the Go code style used across xinkai's projects: names describe semantics not implementation, callers carry zero burden, searchability is first-class. Covers constructor and type naming, interface sizing, hiding implementation behind accessors, the panic-with-typed-error discipline and when to return instead, the GetCurrentXxx singleton pattern, comment style, and repo layout (cmd/ internal/ service/ docs/ research/). Version-free by design — it contains no library API snippets. Also carries the bootstrap checklist for putting a new repo on this stack. Invoke when the user says "onboard honest-go" / "set up this project with honest-go" / "bootstrap a new Go project" / "what do I need for a new project", when starting a Go repo that has no conventions declared yet, when writing or reviewing Go in a project that follows Honest Go, when naming a new type/constructor/interface, when deciding whether a function should panic or return an error, or when the user says "honest go" / "is this honest go" / "review naming".
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

## Repo layout

Two rules earn their place by deriving from the principles above. Everything
else is a default you can depart from with a reason.

**`internal/` holds everything not meant to be imported from outside.** This is
the only form of "hide implementation from callers" that the compiler enforces —
lowercase names, comments, and good intentions do not. Putting a package outside
`internal/` is a promise of API stability to strangers; make that promise
deliberately, for the few packages that are genuinely a public surface, and put
the rest inside. A package that migrates out of `pkg/` into `internal/` is
moving in the right direction.

**`cmd/<binary>/main.go`, where the folder name is the binary name.** Principle
1 at the filesystem level, and it makes "where does this program start" a
one-hop search instead of a grep for `func main`.

**Consistency across repos is itself the third principle at repo scale**: "where
do I find X" should have one answer, not one per project.

### The default layout

```
cmd/<binary>/     one folder per binary, folder name == binary name
internal/         everything not meant to be imported from outside
service/          long-running service assembly
docs/             design docs, notes, diagrams  (docs/, never doc/)
research/         dated investigation write-ups: research/<date>.<topic>/notes.md
skills/           agent skills owned by this repo
bin/              build output (gitignored)
web/              front-end assets, if any
```

`docs/` over `doc/` is settled: the two repos on this stack had drifted apart,
and the migration cost was 2 files with zero references on one side against 163
files with 388 references on the other. GitHub also serves Pages from `/docs`.

### Every module has a Makefile

Not a build system — a **discoverable list of what you can do to this module**.
`make help` should answer "how do I run the tests here" without reading CI
config or asking anyone. Adoption is already total: all 8 module Makefiles in
these repos carry the same core.

```make
all: fmt lint build test    # the pre-commit sweep
build:                      # binaries → bin/ (omit for a library)
test:                       # go test ./... -count=1
fmt:                        # gofmt -w
lint:                       # go vet ./... (+ linter if configured)
tidy:                       # go mod tidy
clean:                      # remove bin/ and test output
run:                        # run the primary binary locally
docker-build: docker-push:  # only if the module ships an image
help:                       # list targets — make this the default if you add nothing else
```

Two names are settled by majority, because both spellings exist in the wild and
a target you have to guess at defeats the point: **`lint`** (7 of 9), not `vet`;
**`docker-build`** (7 of 9), not `docker`. A module may add targets freely —
`test-race`, `test-with-log`, per-binary builds — but not rename these.

Binaries go to `bin/` (7 of 8 Makefiles; the one using `build/` is the outlier).

One deliberate change from current practice: **`test` should pass `-count=1`**.
No existing Makefile does, so this is a proposal rather than a description — Go
caches passing results, so a bare `go test` answers "has anything changed since
the last pass", not "does it pass". The difference shows up exactly when you
most want the truth: re-running after a change elsewhere.

### What is not prescribed

Everything driven by what the project *is* — `ios/`, `k8s/`, `data/`,
`scripts/`, `api/`, domain packages. Add what you need; do not add a folder
because a template has one.

In particular, do not adopt `golang-standards/project-layout` wholesale. It is
not official Go guidance despite the name, and most of its folders are cargo for
any given project. The two rules above are the ones with a reason behind them.

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

**2. Depend on the library** — two modules, not one. The wiring block needs a
metrics exporter, which xklib deliberately does not bundle (it emits through
OpenCensus registries; choosing the exporter is the service's call).

```bash
go get github.com/xinkaiwang/shardmanager/libs/xklib@latest
go get contrib.go.opencensus.io/exporter/prometheus     # or your exporter of choice
```

**3. Wire `main()`** — copy the startup block from the module's `README.md`
§ "Wiring it into main()" **whole**. Four of its six steps fail silently when
omitted (metrics recorded and scraped by nobody, process gauges reading a steady
zero, logs with no `trace_id`, requests starting their own trace). Do not trim
it to "the parts we need".

**Name the smoke service for what it is.** The step-3 binary is a health-check
stub, so call it `hellosvc` — not the project's name. Naming it `kitten` inside
the kitten repo makes `cmd/kitten` a placeholder squatting the name the real
binary will want, and it lies to the next reader about what the folder holds.
Principle 1 applies to binaries too: the name says what the thing *is*. Delete
or rename it once a real entrypoint exists.

Add the module's `Makefile` at the same time (see § Repo layout) — the smoke
service is exactly what `make run` should start.

**4. Prove it compiles and runs.** Not optional, and not "it looks right":

```bash
go mod tidy && go build ./... && go vet ./...
PORT=8080 METRICS_PORT=9090 go run ./cmd/<binary>
curl -s -o /dev/null -w '%{http_code}\n' localhost:8080/health   # expect 200
curl -s localhost:9090/metrics | grep process_goroutines          # expect a real number
```

This step exists because the first repo bootstrapped this way did not build:
`go mod tidy` had run before `main.go` existed, so the module was recorded as
`// indirect` with no `go.sum` entries for its transitive deps, and the exporter
from step 2 was never added at all. Everything read correctly. Nothing compiled.

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
