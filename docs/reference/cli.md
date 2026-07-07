---
title: CLI reference
status: current
summary: Every maat command, flag, and exit code.
related_code:
  - internal/maat/cli.go
---

# CLI reference

Ma'at is a single self-contained binary invoked as `maat <command>`. All
commands take an optional trailing `PATH` argument — the repository root — which
defaults to the current directory.

```
maat init  [--name NAME] [--summary TEXT] [--force] [PATH]
maat sync  [PATH]
maat check [--format text|github] [--strict] [PATH]
maat --version
```

## `init`

Scaffold Ma'at into a repository: writes `AGENTS.md`, the `docs/` tree,
`templates/`, `.maat.yml`, and the CI workflow, then runs `sync` to produce
the derived files.

| Flag | Effect |
|------|--------|
| `--name NAME` | Project name used in generated content (default: directory name) |
| `--summary TEXT` | One-line project summary stamped into `AGENTS.md`/`llms.txt` |
| `--force` | Overwrite existing scaffold files (default: skip files that exist) |

Existing files are never overwritten unless `--force` is given, so re-running
`init` is safe and only fills in what is missing. When files are skipped
(brownfield adoption in an existing repository), `init` prints next-steps
guidance: run `maat check` for the gap list, and point your AI agent at the
scaffolded `.maat/skills/retrospect/SKILL.md` to derive documentation from the
existing codebase. See
[ADR 0008](../decisions/0008-brownfield-adoption-byo-agent.md).

Even when the instruction file (`AGENTS.md`) already exists and is skipped,
`init` still splices Ma'at's **maintenance contract** — the documentation
update protocol, the front-matter schema, and the skills index — into a managed
block inside it, non-destructively (see
[ADR 0009](../decisions/0009-contract-as-managed-block.md)). That is why the
instruction file can be reported as both `skip` and `gen`.

## `sync`

Regenerate every derived artifact from the docs tree: `docs/llms.txt`, the
managed navigation block in `docs/index.md`, the configured agent adapter
files, the managed agent skills (`.maat/skills/` and their vendor copies), and
the managed maintenance-contract block in `AGENTS.md` (update protocol,
front-matter schema, and skills index — ADR 0009). Only files whose content actually changes
are rewritten. Run this after
editing any doc's front-matter, adding/removing a doc, or changing the adapter
list in `.maat.yml`.

## `check`

Validate the documentation set. Prints findings and exits non-zero on errors.
This is the command CI runs.

| Flag | Effect |
|------|--------|
| `--format text` | Human-readable output (default) |
| `--format github` | Emit GitHub Actions `::error`/`::warning` annotations |
| `--strict` | Treat `staleness` warnings as errors |

See the [check engine](../architecture/modules/check.md) for the full rule
list.

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | Success — no error-severity findings |
| `1` | Validation failed — at least one error-severity finding |
| `2` | Usage/configuration error (e.g. no `docs/` directory, bad arguments, or a released binary that does not satisfy the repo's [`maat_version`](configuration.md#maat_version) constraint) |
