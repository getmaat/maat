---
title: Check engine module
status: current
summary: The validation rules behind `maat check` — the CI gate.
related_code:
  - internal/maat/check.go
---

# Check engine

## Responsibility

Validate the documentation set and report problems as structured `Finding`
objects. This is what `maat check` runs and what CI gates merges on.

## Key files

- `internal/maat/check.go` — one function per rule, plus `RunAll()` which
  runs them in order and sorts findings (errors first, then by location).

## The rules

| Rule | Severity (default) | What it catches |
|------|--------------------|-----------------|
| `frontmatter` | error | Missing required keys; unknown `status` value |
| `broken_link` | error | A relative Markdown link whose target file is absent |
| `orphaned_code` | error | A `related_code` path that no longer exists |
| `staleness` | warn | A `related_code` file modified more recently than its doc |
| `drift` | error | A generated file that differs from what `sync` would write |

Severities are configurable in `.maat.yml` under `check:`. `--strict`
promotes `staleness` to an error (useful for a stricter CI lane).

## Interfaces / contracts

- Every rule returns `[]Finding`; `RunAll()` concatenates and sorts them.
- `check` exits `1` if any `error`-severity finding is present, else `0`. A
  missing `docs/` directory exits `2` (misconfiguration, not a doc problem).
- External links (`http://`, `mailto:`, anchors) are skipped by the link
  checker by design — it validates *repository-internal* references only.

## Gotchas

- Staleness uses filesystem mtimes with a 1-second tolerance. After refreshing
  a doc, re-save it (or run `sync`) so its mtime advances past the code's.
- Drift detection reads the current on-disk generated files and compares them
  to freshly generated content after whitespace normalization, so trailing
  newline differences do not cause false positives.
