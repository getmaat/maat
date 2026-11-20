---
title: Contracts lockfile reference
status: current
summary: The format of .maat/contracts.lock.json, the committed snapshot of the contracts a repository consumes from other repositories, and how check uses it.
related_code:
  - internal/maat/lockfile.go
provides:
  - id: contracts-lockfile-format
    kind: schema
    version: 1.0.0
---

# Contracts lockfile reference

`.maat/contracts.lock.json` is the committed snapshot of the contracts this
repository consumes from **other** repositories (see
[ADR 0014](../decisions/0014-cross-repo-contracts-via-lockfile.md)). It is
written by `maat contracts update`, the only Ma'at command that uses the
network, and read by `maat check`, which validates cross-repo `consumes` refs
against it **offline**. `sync` never writes it.

It exists only while the repository consumes at least one cross-repo contract.

## Configuring sources

Every repository named in a cross-repo ref (`owner/repo#id`) needs an entry
under `contracts.sources` in `.maat.yml`:

```yaml
contracts:
  sources:
    getmaat/ledger:                 # no value: GitHub, default branch
    getmaat/billing:
      ref: release                  # branch, tag or commit
    acme/payments:
      url: https://git.acme.example/raw/{repo}/{ref}/.maat/manifest.json
      ref: main
    acme/sandbox:
      path: ../sandbox              # a sibling checkout, no network
```

| Key | Meaning |
|-----|---------|
| `ref` | Branch, tag or commit to read. Default: the default branch. |
| `url` | URL template for hosts other than GitHub; `{repo}` and `{ref}` are filled in. Must be `https://`. |
| `path` | Read `.maat/manifest.json` from a local checkout instead of fetching. |

Set `url` or `path`, not both. Write an empty value (`owner/name:`) for all
defaults — not `{}`, which the YAML subset reads as a string. Credentials come
only from the environment (`GITHUB_TOKEN` / `GH_TOKEN`, or `MAAT_TOKEN` for
`url` sources), never from `.maat.yml`.

## Format

```json
{
  "schema": 1,
  "sources": {
    "getmaat/ledger": {
      "commit": "3f9c2e1d…",
      "manifest_schema": 1,
      "provides": [
        {"id": "ledger-events", "kind": "event", "version": "1.6.0",
         "doc": "docs/architecture/modules/events.md"}
      ],
      "missing": ["ledger-api"]
    }
  }
}
```

| Field | Meaning |
|-------|---------|
| `schema` | Lockfile format version, currently `1`. |
| `sources` | One entry per consumed repository, keyed `owner/name`. |
| `ref` / `url` / `path` | The source settings the snapshot was taken with (omitted when unset). |
| `commit` | The provider commit at which this state of the consumed contracts was first observed (GitHub sources only). It is not updated for provider commits that change nothing consumed. |
| `manifest_schema` | The provider manifest's `schema`. |
| `provides` | The **consumed** contracts the provider has, as declared in its manifest. |
| `missing` | Consumed ids the provider does not provide. |

Only consumed contracts are recorded, never the whole remote manifest, so the
lockfile's diff shows exactly the contracts that changed. It contains no
timestamps: updating with no remote changes leaves it untouched.

## What `check` reports

| Situation | Rule |
|-----------|------|
| A cross-repo ref names a repository with no `contracts.sources` entry | `contract_schema` |
| The provider does not provide the id (it is in `missing`) | `contract_unresolved` |
| The locked version does not satisfy the constraint, or there is none | `contract_version` |
| The lockfile is missing, unreadable, or of an unsupported schema | `drift` |
| A ref, source or source setting changed since the last update | `drift` |
| The lockfile lists a repository that is no longer consumed, or exists with nothing consumed | `drift` |

Every `drift` finding is fixed by running `maat contracts update`. Severities
follow `check.contracts_is_error` and `check.drift_is_error`.

A ref to this repository's own `repo` (e.g. `getmaat/maat#maat-cli` inside
`getmaat/maat`) is resolved locally and needs no source.

## Staying current

`check` cannot see provider changes made after the last update; that is the
price of an offline, deterministic check. Run
`maat contracts update --check` on a schedule (daily, say) in CI: it fails
when the lockfile is out of date, and a human runs `maat contracts update` and
commits the diff.
