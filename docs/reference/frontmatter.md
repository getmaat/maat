---
title: Front-matter reference
status: current
summary: The YAML front-matter schema every doc in docs/ carries.
related_code:
  - internal/maat/model.go
  - internal/maat/frontmatter.go
---

# Front-matter reference

Every Markdown file under `docs/` begins with a YAML front-matter block
delimited by `---`. It carries the metadata Ma'at uses to build indexes and
run validation.

```markdown
---
title: Check engine module
status: current
summary: The validation rules behind `maat check`.
related_code:
  - internal/maat/check.go
---

# Check engine
...
```

## Fields

| Field | Required | Type | Purpose |
|-------|----------|------|---------|
| `title` | yes | string | Human-readable title; used in `llms.txt` and index nav |
| `status` | yes | enum | `current`, `draft`, or `deprecated` |
| `summary` | recommended | string | One line shown next to the doc in generated indexes |
| `related_code` | optional | list of paths | Source files this doc describes; enables staleness + orphan checks |

Which fields are *required* is configurable via `required_frontmatter` in
`.maat.yml` (default: `title`, `status`). The allowed `status` values are
configurable via `statuses`.

## `related_code`

Paths are relative to the repository root and point at the source this document
describes. They power two checks (see
[ADR 0004](../decisions/0004-related-code-staleness.md)):

- **orphaned_code** — the path no longer exists (error by default).
- **staleness** — the source file is newer than this doc (warning by default;
  error under `--strict`).

List the files a reader of this doc would need to keep in sync with it. Omit
the field entirely for docs that do not describe specific source files (guides,
glossary, ADRs about process).

## Notes

- The parser accepts a YAML subset (see the
  [YAML module](../architecture/modules/yaml-frontmatter.md)); keep values to
  scalars and simple lists.
- An empty or absent front-matter block is legal YAML-wise but will fail the
  `frontmatter` check if required keys are missing.
