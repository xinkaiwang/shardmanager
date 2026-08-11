# Agent skills owned by this repo

Source of truth for agent skills that belong to this codebase — version
controlled, reviewable in a PR, and shared instead of living only in someone's
home directory.

**Whether this folder is itself a discovery path depends on the runtime.**
OpenClaw scans `<workspace>/skills` and gives it the highest precedence
(`<workspace>/skills` > `~/.openclaw/skills` > bundled), so a skill here loads
with no install at all — but only while this repo is the workspace. Claude Code
and the `~/.agents` family never scan a repo. Symlink into each runtime's own
directory to get every agent, in every project, from this one copy.

## Install

```bash
# from the repo root
for s in honest-go using-xklib research-discipline; do
  ln -s "$PWD/skills/$s" ~/.claude/skills/$s                        # Claude Code
  mkdir -p ~/.agents/skills   && ln -s "$PWD/skills/$s" ~/.agents/skills/$s    # Codex / Copilot CLI / Gemini CLI
  mkdir -p ~/.openclaw/skills && ln -s "$PWD/skills/$s" ~/.openclaw/skills/$s  # OpenClaw (global; workspace copies still win)
done
```

All three coexist; each runtime reads the one it knows, and every one resolves
to the same file. **Do not copy a skill into another project's `skills/`** — that
creates a second copy to keep in sync, which is the failure this layout exists
to prevent. The global symlink already reaches every project.

| Runtime | Where it looks |
| --- | --- |
| Claude Code | `~/.claude/skills/` |
| Codex, Copilot CLI, Gemini CLI | `~/.agents/skills/` |
| OpenClaw | `<workspace>/skills` > `~/.openclaw/skills` > bundled; plus `skills.load.extraDirs` in `~/.openclaw/openclaw.json` |

## Catalog

| Skill | What it does |
| --- | --- |
| [`honest-go`](honest-go/SKILL.md) | The Go **style**: semantic names, zero caller burden, searchability first-class, interface sizing, hiding implementation, panic-with-typed-error discipline, `GetCurrentXxx` singletons, comment style. Version-free — contains no library API. |
| [`using-xklib`](using-xklib/SKILL.md) | Authoring rules for code that depends on xklib: ctx-carrying logs, the `event` convention, the kcommon time abstraction, kerror, kmetrics, and the krunloop adopt-or-not decision. Fires **before** code is written, unlike the scan-only `smell-*` family. |
| [`research-discipline`](research-discipline/SKILL.md) | How dated design docs, investigation notes, and numbered experiment folders are laid out — and the rule that a rerun with a changed parameter is a new experiment, never an overwrite. |

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
