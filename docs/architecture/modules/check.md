---
title: Check engine module
status: current
summary: The validation rules behind `maat check` — the CI gate.
related_code:
  - internal/maat/check.go
  - internal/maat/staleness.go
---

# Check engine

## Responsibility

Validate the documentation set and report problems as structured `Finding`
objects. This is what `maat check` runs and what CI gates merges on.

## Key files

- `internal/maat/check.go` — one function per rule, plus `RunAll()` which
  runs them in order and sorts findings (errors first, then by location).
- `internal/maat/staleness.go` — the staleness probes: `gitProbe` (commit
  order plus uncommitted changes) and `mtimeProbe` (the fallback), chosen by
  `check.staleness_source` ([ADR 0012](../../decisions/0012-git-aware-staleness.md)).

## The rules

| Rule | Severity (default) | What it catches |
|------|--------------------|-----------------|
| `frontmatter` | error | Missing required keys; unknown `status` value |
| `broken_link` | error | A relative Markdown link whose target file is absent |
| `orphaned_code` | error | A `related_code` path that no longer exists |
| `staleness` | warn | A `related_code` path changed after its doc was last updated (git), or has a newer mtime (non-git fallback) |
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

- In a git work tree, staleness compares commit order: a doc is stale when a
  commit after its last change touched its `related_code`. Uncommitted edits
  count too (dirty code + clean doc = stale; dirty doc = being refreshed).
  Clear a finding by updating the doc in the same change.
- A shallow clone hides older commits, so `check` warns once (against
  `.maat.yml`) and CI should use `fetch-depth: 0`.
- Outside git, staleness falls back to filesystem mtimes with a 1-second
  tolerance; re-save a refreshed doc so its mtime advances past the code's.
- Git runs as an external command; without it, `auto` mode silently uses mtimes.
- Drift detection reads the current on-disk generated files and compares them
  to freshly generated content after whitespace normalization, so trailing
  newline differences do not cause false positives.
