---
title: Contracts update module
status: current
summary: How maat contracts update fetches provider manifests and writes the lockfile — the only networked code in Ma'at.
related_code:
  - internal/maat/contracts_update.go
  - internal/maat/fetch.go
---

# Contracts update

## Responsibility

Implement `maat contracts update [--check]`
([ADR 0014](../../decisions/0014-cross-repo-contracts-via-lockfile.md)):
read the manifests of the repositories this one consumes contracts from, and
write — or with `--check`, only compare — `.maat/contracts.lock.json`.

This is the **only** code in Ma'at that uses the network. It is never called
by `sync`, `check` or `init`; validating cross-repo refs against the lockfile
is the check engine's job (`lockfile.go`, see [Check engine](check.md)).

## Key files

- `internal/maat/contracts_update.go`
  - `consumedByRepo()` — cross-repo consumed ids per provider, skipping refs
    to this repository's own `repo`.
  - `buildLock()` — fetches every source, builds the new lockfile, and
    collects **all** errors rather than stopping at the first.
  - `sameSignificant()` — decides whether a source's entry may be kept as is.
  - `describeLockChanges()` — the per-contract change list printed by both
    modes.
  - `cmdContracts()` — the command: dispatch, write/remove, or report.
- `internal/maat/fetch.go`
  - `fetchManifest()` — one source: `path`, `url` template, or GitHub.
  - `fetchFromGitHub()` — resolves the ref to a commit first, then reads the
    manifest **at that commit**, so the two always agree.
  - `httpGet()` — the single HTTP entry point; bounds response size.

## Interfaces / contracts

- **All or nothing.** If any source fails, nothing is written and the exit
  code is `1`; the previous lockfile stays as it was.
- **Only significant changes rewrite a source's entry.** Significant means:
  source settings, manifest schema, which consumed ids exist, and their
  versions. When none of those changed, the previous entry — including its
  `commit`, `kind` and `doc` — is kept, so a provider's unrelated commits never
  churn the lockfile or fail `--check` (ADR 0014, review question 2). The
  recorded `commit` therefore means "where this state was first observed".
- **Tokens:** GitHub sources send `GITHUB_TOKEN` or `GH_TOKEN`; `url` sources
  send only `MAAT_TOKEN`. Tokens are never printed or written. Go's HTTP
  client drops the `Authorization` header on cross-host redirects.
- `GITHUB_API_URL` overrides the GitHub API root (set by GitHub Actions,
  including on GitHub Enterprise Server); `HTTPS_PROXY` is honored.
- Exit codes: `0` written / up to date, `1` fetch failure or (`--check`) out
  of date, `2` usage or configuration error.

## Gotchas

- Tests never touch the real network: they run a fake GitHub API via
  `httptest` and point `GITHUB_API_URL` at it, and swap the package-level
  `httpClient` to trust a TLS test server for `url` sources.
- A provider manifest with a **newer** `schema` is refused with an upgrade
  hint rather than half-understood.
- Adding a network call anywhere else breaks the offline guarantee of `sync`
  and `check`; it needs its own ADR.
