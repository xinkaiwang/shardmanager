# Agent skills owned by this repo

Source of truth for agent skills that belong to this codebase — version
controlled, reviewable in a PR, and shared instead of living only in someone's
home directory.

**This folder is not a discovery path.** No agent runtime loads skills from
`<repo>/skills/`; they load from the runtime's own directory. Install by
symlink, so edits here take effect immediately with no copy to keep in sync.

## Install

```bash
# Claude Code
for s in honest-go using-xklib; do ln -s "$PWD/skills/$s" ~/.claude/skills/$s; done

# Codex / Copilot CLI / Gemini CLI (cross-runtime alias)
mkdir -p ~/.agents/skills && for s in honest-go using-xklib; do ln -s "$PWD/skills/$s" ~/.agents/skills/$s; done
```

Both locations can coexist; each runtime reads the one it knows.

## Catalog

| Skill | What it does |
| --- | --- |
| [`honest-go`](honest-go/SKILL.md) | The Go **style**: semantic names, zero caller burden, searchability first-class, interface sizing, hiding implementation, panic-with-typed-error discipline, `GetCurrentXxx` singletons, comment style. Version-free — contains no library API. |
| [`using-xklib`](using-xklib/SKILL.md) | Authoring rules for code that depends on xklib: ctx-carrying logs, the `event` convention, the kcommon time abstraction, kerror, kmetrics, and the krunloop adopt-or-not decision. Fires **before** code is written, unlike the scan-only `smell-*` family. |

## Where a rule belongs

One question decides it, and following it is what keeps these docs from rotting:

> **Would this line have to change when xklib releases a new version?**

| Answer | Home | Why |
| --- | --- | --- |
| Yes | `libs/xklib/AGENTS.md`, `libs/xklib/UPGRADING.md` | Ships in the module zip, so it always matches the version the consumer resolved. |
| No, and it holds in every Go project here | `skills/honest-go/` | One copy, symlinked into the runtime dirs. |
| No, and it holds only in one project | That project's `CLAUDE.md` | Never duplicated outward. |

Content lives in exactly one place; everything else points at it. The rule was
written after finding a project-local style doc that had been teaching a deleted
xklib function for months — version-bound content copied into a version-free
document, which is the failure this split prevents.

## Note on distribution

These skills do **not** ship inside the xklib Go module — only files under
`libs/xklib/` land in the module zip. A project that runs `go get` gets
[`libs/xklib/AGENTS.md`](../libs/xklib/AGENTS.md) (the rules themselves) but not
the skill wrapper. The skill is the trigger; AGENTS.md is the content, and it is
the one that has to travel with the version.

To use these skills from another project, clone this repo and symlink as above.
