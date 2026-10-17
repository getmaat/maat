---
title: Contract manifest reference
status: current
summary: The format of .maat/manifest.json, the generated index of a repository's contract edges.
related_code:
  - internal/maat/contracts.go
provides:
  - id: manifest-format
    kind: schema
    version: 1.0.0
---

# Contract manifest reference

`maat sync` writes `.maat/manifest.json`: every `provides` and `consumes` entry
declared in the docs' front-matter, so other repositories and tools can learn
what this repository offers and depends on without parsing Markdown (see
[ADR 0013](../decisions/0013-contract-edges-and-manifest.md)). It is a
generated file: `maat check` reports it as `drift` when it is out of date.

The path is fixed — not under `docs_dir`, which is configurable — so it can
always be found at the same place in any repository that uses Ma'at.

## Format

```json
{
  "schema": 1,
  "repo": "getmaat/billing",
  "provides": [
    {
      "id": "invoice-api",
      "kind": "http-api",
      "version": "2.1.0",
      "spec": "api/openapi.yaml",
      "doc": "docs/architecture/modules/invoice-api.md"
    }
  ],
  "consumes": [
    {
      "ref": "getmaat/ledger#ledger-events",
      "version": "~> 1.4",
      "doc": "docs/architecture/modules/ledger-client.md"
    }
  ]
}
```

| Field | Meaning |
|-------|---------|
| `schema` | Manifest format version. Currently `1`; it changes only on an incompatible change. |
| `repo` | The `repo` value from `.maat.yml`, or `null` when unset. |
| `provides[].id` | Contract id, unique within the repository. Globally addressed as `repo#id`. |
| `provides[].kind` | Free-form kind as declared (omitted when not set). |
| `provides[].version` | Semver as declared (omitted when not set). |
| `provides[].spec` | Repository path of a machine-readable spec (omitted when not set). |
| `provides[].doc` | The doc that declares and explains the contract. |
| `consumes[].ref` | A contract id in this repository, or `owner/repo#id` in another. |
| `consumes[].version` | Version constraint as declared (omitted when not set). |
| `consumes[].doc` | The doc that declares the dependency. |

## Guarantees

- Entries are sorted (`provides` by `id`, `consumes` by `ref`, then `doc`) and
  the JSON is indented, so the file diffs cleanly and regenerates byte-for-byte.
- Only entries that pass validation are included; invalid ones are reported by
  `maat check` as `contract_schema` findings instead.
- The file is always present after `sync`, with empty lists when nothing is
  declared.
- Keys may be added in later versions without changing `schema`; consumers
  should ignore keys they do not know.
