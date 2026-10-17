---
title: Front-matter reference
status: current
summary: The YAML front-matter schema every doc in docs/ carries.
related_code:
  - internal/maat/model.go
  - internal/maat/frontmatter.go
  - internal/maat/contracts.go
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
| `provides` | optional | list of mappings | Contracts this doc describes (an API, event, schema, CLI…); see below |
| `consumes` | optional | list of mappings | Contracts this doc's subject depends on; see below |

Which fields are *required* is configurable via `required_frontmatter` in
`.maat.yml` (default: `title`, `status`). The allowed `status` values are
configurable via `statuses`.

## `related_code`

Paths are relative to the repository root and point at the source this document
describes. They power two checks (see
[ADR 0004](../decisions/0004-related-code-staleness.md) and
[ADR 0012](../decisions/0012-git-aware-staleness.md)):

- **orphaned_code** — the path no longer exists (error by default).
- **staleness** — the source changed after this doc was last updated (judged
  from git history, or file mtimes outside git; warning by default, error
  under `--strict`).

List the files a reader of this doc would need to keep in sync with it. Omit
the field entirely for docs that do not describe specific source files (guides,
glossary) and for **ADRs**: decisions are append-only records, so tying them to
code would flag them as stale on every later change.

## `provides` and `consumes`

Typed contract edges (see
[ADR 0013](../decisions/0013-contract-edges-and-manifest.md)). A doc that
describes an interface declares it with `provides`; a doc whose subject depends
on one declares it with `consumes`:

```yaml
provides:
  - id: invoice-api            # required; lowercase letters, digits, dashes
    kind: http-api             # optional, free-form (http-api, event, schema, cli…)
    version: 2.1.0             # optional semver
    spec: api/openapi.yaml     # optional path to a machine-readable spec
consumes:
  - ref: invoice-api                     # a contract provided in this repo
    version: "~> 2.1"                    # optional constraint
  - ref: getmaat/ledger#ledger-events    # owner/repo#id for another repository
```

- `id` must be unique across the repository.
- `spec` is treated like a `related_code` path: it must exist, and the doc is
  stale when the spec changes after it.
- `version` constraints use the same syntax as `maat_version` (`~> 2.1`,
  `>= 2.0, < 3`). **Quote versions with two parts** (`"1.10"`): unquoted, YAML
  reads them as numbers, and `check` reports them instead of guessing.
- Same-repo refs are checked: they must resolve to a `provides` entry
  (`contract_unresolved`) whose version satisfies the constraint
  (`contract_version`). Cross-repo refs are recorded in the manifest but not
  resolved yet.
- Malformed entries (missing `id`/`ref`, unknown keys, bad versions,
  duplicate ids) are reported as `contract_schema`.

Every valid edge is collected into the generated
[contract manifest](manifest.md) and listed in `llms.txt`.

## Notes

- The parser accepts a YAML subset (see the
  [YAML module](../architecture/modules/yaml-frontmatter.md)); keep values to
  scalars, lists, and lists of simple mappings.
- An empty or absent front-matter block is legal YAML-wise but will fail the
  `frontmatter` check if required keys are missing.
