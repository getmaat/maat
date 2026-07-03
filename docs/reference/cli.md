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
`init` is safe and only fills in what is missing.

## `sync`

Regenerate every derived artifact from the docs tree: `docs/llms.txt`, the
managed navigation block in `docs/index.md`, and the configured agent adapter
files. Only files whose content actually changes are rewritten. Run this after
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
| `2` | Usage/configuration error (e.g. no `docs/` directory, bad arguments) |
