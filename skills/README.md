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
ln -s "$PWD/skills/using-xklib" ~/.claude/skills/using-xklib

# Codex / Copilot CLI / Gemini CLI (cross-runtime alias)
mkdir -p ~/.agents/skills && ln -s "$PWD/skills/using-xklib" ~/.agents/skills/using-xklib
```

Both locations can coexist; each runtime reads the one it knows.

## Catalog

| Skill | What it does |
| --- | --- |
| [`using-xklib`](using-xklib/SKILL.md) | Authoring rules for code in a project that depends on xklib: ctx-carrying logs, the `event` convention, the kcommon time abstraction, kerror, kmetrics, and the krunloop adopt-or-not decision. Fires **before** code is written, unlike the scan-only `smell-*` family. |

## Note on distribution

These skills do **not** ship inside the xklib Go module — only files under
`libs/xklib/` land in the module zip. A project that runs `go get` gets
[`libs/xklib/AGENTS.md`](../libs/xklib/AGENTS.md) (the rules themselves) but not
the skill wrapper. The skill is the trigger; AGENTS.md is the content, and it is
the one that has to travel with the version.

To use these skills from another project, clone this repo and symlink as above.
