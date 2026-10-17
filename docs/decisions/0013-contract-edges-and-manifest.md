---
title: 0013 Typed contract edges in front matter and a generated manifest
status: draft
summary: Docs declare the contracts they describe (provides) and depend on (consumes); sync generates a machine-readable manifest at .maat/manifest.json; check validates edges within the repo. Groundwork for cross-repo checks.
---

# 0013. Typed contract edges in front matter and a generated manifest

- **Status:** proposed
- **Date:** 2026-10-17
- **Relates to:** [0004](0004-related-code-staleness.md) / [0012](0012-git-aware-staleness.md) (`related_code` and staleness), [0007 Agent skills as managed artifacts](0007-agent-skills-as-managed-artifacts.md) (`.maat/` as the home for tool-owned files)

## Context

Ma'at keeps docs, code and agent instructions in sync **within one
repository**. Real systems span many: a service's docs describe an API that
three other repos call, and an agent working in one of those callers has no
reliable way to learn what the API guarantees, which version it is on, or that
it changed. Teams paper over this with copied snippets in `AGENTS.md`, which
then drift with nobody noticing — the exact failure Ma'at exists to prevent,
one level up.

Two things are missing:

1. **A way to say what a doc is a contract for.** `related_code` links a doc
   to source files, but not to the *interfaces* those files expose, and says
   nothing about what the repo depends on.
2. **A machine-readable artifact another repo or tool can read** to learn that,
   without parsing Markdown.

A general knowledge graph (inferred entities, embeddings) was considered and
is the obvious first idea. It is rejected below: the edges that matter are
few, and an inferred graph would drift exactly like hand-copied docs do. What
is needed is a small, *declared* graph whose edges are reviewed in pull
requests like any other doc change.

This ADR covers the single-repo groundwork only. Resolving edges *across*
repositories needs network access, which the coding conventions currently
rule out; that is a separate decision (a future ADR).

## Decision

### 1. Two new optional front-matter fields

A doc that describes a contract declares it with `provides`; a doc that
describes a dependency declares it with `consumes`. Both are lists of
mappings:

```yaml
---
title: Invoice API
status: current
related_code:
  - internal/billing/api.go
provides:
  - id: invoice-api            # required; [a-z0-9][a-z0-9-]*, unique per repo
    kind: http-api             # optional; free-form, lowercase (see below)
    version: 2.1.0             # optional; semver
    spec: api/openapi.yaml     # optional; repo path to a machine-readable spec
consumes:
  - ref: getmaat/ledger#ledger-events   # required; [owner/repo#]id
    version: "~> 1.4"                    # optional; constraint
---
```

- **`id`** names the contract within the repository. Together with the repo
  identity (below) it forms the global address `owner/repo#id`.
- **`kind`** is free-form so Ma'at stays language- and protocol-agnostic.
  Suggested values: `http-api`, `grpc-api`, `event`, `schema`, `library`,
  `cli`. It is descriptive only and changes no check, so an enum would add
  friction without catching anything.
- **`version`** is plain semver, parsed with the existing `semver.go`.
- **`spec`** points at an OpenAPI/AsyncAPI/JSON-Schema/proto file. Ma'at does
  not interpret it. It is validated like a `related_code` path (must exist) and
  **counts as related code for staleness**: when the spec changes after the
  doc, the doc is stale.
- **`ref`** without `owner/repo#` refers to a contract provided in the same
  repository. With it, the edge points at another repository.
- **`version`** on a `consumes` entry uses the constraint syntax `maat_version`
  already uses (`~> 2.1`, `>= 2.0, < 3`), so there is one version language.

### 2. Repository identity

A new optional `.maat.yml` key, `repo: owner/name`, names the repository in
global addresses. It is optional in this release (recorded as `null` in the
manifest when absent) and becomes required only once cross-repo resolution
exists.

It is **configured, never derived at runtime**. A fallback to the git
`origin` remote would make the identity depend on where `check` runs: a fork,
a mirror or a CI checkout with a rewritten remote would each produce a
different `repo`, so the generated manifest would differ between machines and
the drift check would flip-flop. An identity other repositories point at must
be a committed fact. For convenience, `maat init` **pre-fills** `repo` once
from the `origin` remote when it can parse it; the value is written to
`.maat.yml`, reviewed and committed like any other change, and never
re-derived afterwards.

### 3. A generated manifest at `.maat/manifest.json`

`maat sync` writes `.maat/manifest.json`: the repo identity plus every
`provides` and `consumes` entry, each with the path of the doc that declares
it. Output is sorted and stable, so it diffs cleanly. `maat check` reports it
as `drift` like any other generated file.

```json
{
  "schema": 1,
  "repo": "getmaat/billing",
  "provides": [
    {"id": "invoice-api", "kind": "http-api", "version": "2.1.0",
     "spec": "api/openapi.yaml", "doc": "docs/architecture/modules/invoice-api.md"}
  ],
  "consumes": [
    {"ref": "getmaat/ledger#ledger-events", "version": "~> 1.4",
     "doc": "docs/architecture/modules/ledger-client.md"}
  ]
}
```

It lives under `.maat/` rather than `docs/` because `docs_dir` is
configurable, while another repository or tool needs a **fixed, discoverable
path**. `.maat/` is already where Ma'at keeps tool-owned files (ADR 0007).
JSON (via the standard library's `encoding/json`) is used because the
consumers are programs; Ma'at's own YAML layer is read-only by design.

### 4. Contracts in `llms.txt`

`sync` adds a `## Contracts` section to `llms.txt` listing what the repo
provides and consumes, each linked to its doc, so agents see the interfaces
before reading individual pages.

### 5. Checks (all local)

| Rule | Default | Catches |
|------|---------|---------|
| `contract_schema` | error | missing `id`/`ref`; invalid `id`; duplicate `id` in the repo; invalid semver or constraint; unknown keys |
| `orphaned_code` (extended) | error | a `spec` path that does not exist |
| `contract_unresolved` | error | a same-repo `consumes` whose `ref` matches no `provides` |
| `contract_version` | error | a same-repo `consumes` whose constraint the provided `version` does not satisfy |

Cross-repo `consumes` are recorded in the manifest and **not checked** in
this release. Severities follow the existing pattern
(`check.contracts_is_error: true`).

### 6. Prerequisite: fix silent data loss in the YAML parser

The parser currently mis-reads a list item that starts with `key: value`
(`- id: invoice-api` followed by indented keys): it stores the first line as
the string `"id: invoice-api"` and **silently drops** the remaining keys,
later list items, and any following top-level keys, without an error. Any
user writing ordinary YAML today can hit this. Before this ADR is
implemented, the parser must support the compact list-of-mappings form, and
any construct it still cannot parse must fail with an error instead of being
dropped. That fix ships first, as its own change, because it matters
independently of this ADR.

## Consequences

- Each repo gains a declared, reviewed contract index that agents read via
  `llms.txt` and tools read via `.maat/manifest.json`.
- A spec change now marks the doc describing it as stale — the doc and the
  machine-readable contract can no longer drift apart silently.
- Same-repo dependency mistakes (renamed contract, incompatible version bump)
  fail CI.
- The manifest is the input for cross-repo checks later: a consumer compares
  its constraints against providers' manifests, and a provider can list who
  consumes it. Nothing here commits to how manifests are fetched.
- Adoption is opt-in per doc. Repos that declare nothing get an empty
  manifest and no new findings.
- Two new front-matter fields, one config key, one generated file and three
  rules to document and maintain.

## Alternatives considered

- **Inferred knowledge graph (entity extraction, embeddings).** Covers more
  without any authoring, but its edges are guesses that nobody reviews, so it
  drifts the same way hand-copied docs do — and Ma'at's whole premise is that
  durable knowledge is declared and checked. Rejected; a graph can still be
  *built from* manifests later.
- **A separate contracts file (`.maat/contracts.yml` or similar).** One place
  to look, but the contract would live apart from the doc that explains it,
  and nothing would connect a contract change to that doc. Front matter keeps
  declaration, explanation and staleness in one reviewed unit.
- **Backstage `catalog-info.yaml` (`providesApis` / `consumesApis`).** An
  existing, widely used format with the same idea. Rejected as the *source*
  because it models services, not documents, and lives in its own file. Worth
  supporting as an **export target** later, since teams that run Backstage
  could then get the catalog for free.
- **Extracting contracts from OpenAPI/AsyncAPI directly.** Precise for HTTP
  and events, but format-specific and blind to libraries, CLIs and schemas.
  The `spec` field links to those files instead of replacing them.
- **String shorthand (`- invoice-api@2.1.0`).** Would work with today's
  parser, but packing several fields into one string invents a mini-syntax
  and leaves no room for `kind` or `spec`. Rejected in favor of plain YAML
  mappings, which also forces the parser fix that is needed anyway.

## Resolved during review

1. **`kind`: free-form, not an enum.** It drives no check; see Decision §1.
2. **Manifest location: `.maat/manifest.json`.** Keeps Ma'at's footprint in
   adopting repositories inside the directory it already owns, instead of
   adding another root-level file; see Decision §3.
3. **`repo`: configured, pre-filled by `init`, never derived at runtime.**
   See Decision §2.
