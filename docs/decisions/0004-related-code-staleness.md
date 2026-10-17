---
title: 0004 Detect staleness via related_code timestamps
status: deprecated
summary: Superseded by ADR 0012. Staleness now uses git history; the mtime comparison remains only as the fallback for non-git directories.
related_code:
  - internal/maat/check.go
---

# 0004. Detect staleness via `related_code` timestamps

- **Status:** deprecated — superseded by [0012 Detect staleness from git history](0012-git-aware-staleness.md)
- **Date:** 2026-07-02

> **Superseded (2026-10-17).** `related_code` and the orphaned-code check stand
> as decided here. The mtime comparison is no longer the primary staleness
> signal: inside a git work tree, `check` compares commit order instead (ADR
> 0012). mtimes are used only for plain directories or with
> `check.staleness_source: mtime`.

## Context

The central failure mode of documentation is silent staleness: code changes,
the doc that describes it does not, and nobody notices until the doc misleads
someone. We want CI to catch this the way it catches a failing test — but
Ma'at is language-agnostic and cannot parse every codebase to know whether a
doc is *semantically* current.

## Decision

Each doc may declare a `related_code` list in its front-matter naming the
source paths it describes. `maat check` compares modification times: if any
listed source file is newer than the doc, it raises a **staleness** finding.

Staleness defaults to a **warning** (visible but non-blocking) and can be
promoted to an error via `.maat.yml` or the `--strict` flag. Two adjacent
checks reuse the same `related_code` data: **orphaned_code** (a listed path no
longer exists) is an error by default.

## Consequences

- Staleness detection is cheap, language-agnostic, and needs no code parsing.
- It is a *heuristic*: touching a file's mtime without meaningfully changing it
  can raise a false positive, and editing a doc without editing code clears the
  flag whether or not the doc is truly correct. This is why the default is a
  warning and why a human still reviews docs in the PR.
- The workflow to clear a real staleness flag is: update the doc, then save it
  (or run `sync`) so its mtime advances past the code's.
- `related_code` doubles as machine-readable traceability from docs to source.

## Alternatives considered

- **Content hashing** of the referenced code stored in the doc — precise about
  *whether* code changed, but noisy (every trivial edit trips it) and it bloats
  the doc with hashes. Rejected as over-engineered for a signal that a human
  reviews anyway.
- **Git-based diffing** in CI (did the PR touch code under a doc's paths without
  touching the doc?) — powerful and a good *future* enhancement, but it depends
  on git history and PR context, so it cannot be the baseline mechanism that
  also works for a local `maat check` on a plain directory.
