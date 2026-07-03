---
title: Documentation conventions
status: current
summary: The rules this documentation set follows so it stays consistent.
---

# Documentation conventions

- **Language:** English, always.
- **One topic per file.** Prefer many small pages over few large ones.
- **Front-matter is required** on every page: at minimum `title` and `status`.
  Add `summary` (used in indexes) and `related_code` (enables staleness
  detection) wherever they apply.
- **Status values:** `current`, `draft`, `deprecated`.
- **Link by relative path** to other docs so links survive moves and are
  validated by `maat check`.
- **Generated regions** (between `<!-- maat:begin -->` and
  `<!-- maat:end -->`) and generated files (`llms.txt`, adapter files) are
  produced by `maat sync` — never hand-edit them.
- **Decisions are append-only.** See `docs/decisions/`.
