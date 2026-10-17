---
title: 0012 Detect staleness from git history
status: current
summary: Staleness compares commit order (and uncommitted changes) via git when available; filesystem mtimes remain only as the fallback for plain directories.
related_code:
  - internal/maat/staleness.go
  - internal/maat/check.go
---

# 0012. Detect staleness from git history

- **Status:** accepted — supersedes [0004 Detect staleness via related_code timestamps](0004-related-code-staleness.md)
- **Date:** 2026-10-17

## Context

[ADR 0004](0004-related-code-staleness.md) detects stale docs by comparing
filesystem modification times of a doc and its `related_code`. In practice that
signal is weakest exactly where it matters most:

- **In CI it barely fires.** A fresh `actions/checkout` writes every file at
  roughly the same moment, so code and doc mtimes are nearly equal and code
  only looks "newer" when its write happens to land more than a second after
  the doc's. The gate meant to catch drift is mostly silent there.
- **Locally it is noisy.** Formatters, branch switches and rebases change
  mtimes without changing meaning, producing false positives.
- **It is easy to game.** The prescribed fix — re-save the doc to bump its
  mtime — clears the finding without anyone reading the doc. AI agents follow
  that instruction literally.

ADR 0004 already named git-based detection as the better future mechanism and
rejected it only as a *baseline*, because a local `maat check` must also work
on a plain directory.

## Decision

We will detect staleness from git history whenever the checked root is inside
a git work tree, and keep mtimes only as the fallback.

- **Committed history:** a doc is stale when a commit reachable from `HEAD`
  touched one of its `related_code` paths *after* the last commit that touched
  the doc (`git rev-list -1 <last-doc-commit>..HEAD -- <path>`). A code change
  committed together with its doc is never stale — this is the update protocol
  ("docs in the same change") checked directly.
- **Uncommitted work:** a doc with local edits counts as being refreshed now; a
  code path with local edits whose doc has none counts as newer than the doc.
- **Shallow clones:** history beyond the clone depth is invisible, so `check`
  emits one `staleness` finding against `.maat.yml` asking for
  `fetch-depth: 0`. The scaffolded CI workflow checks out full history.
- **Selection:** the new `check.staleness_source` key is `auto` (default — git
  when available, else mtime), `git` (require git; an error finding otherwise),
  or `mtime` (the ADR 0004 heuristic, unchanged).

Git is invoked as an external command via `os/exec`. This is a deliberate,
narrow exception to the "no subprocess dependency" coding convention: git is
an **optional enhancement, not a dependency**. `maat` still runs everywhere
without it — `auto` silently degrades to mtimes — and the only place git is
required is when a repository explicitly opts into `staleness_source: git`.
Any directory with git history already has git installed, which is what keeps
this from reintroducing the install friction
[ADR 0011](0011-build-time-go-dependencies.md) guards against.

## Consequences

- Staleness now works in CI, which is where the gate exists.
- mtime-only changes no longer raise findings in git repositories.
- Clearing a finding requires a commit that touches the doc — a reviewable
  diff — instead of an invisible timestamp bump. A doc that is still accurate
  after a code change needs some edit (or the finding stays a warning).
- Repositories that adopt Ma'at on top of existing history may see a burst of
  staleness warnings for old drift that mtimes never revealed. That is the
  check working; it stays a warning by default.
- `check` now runs a few git commands per `related_code` entry. Results are
  cached per path; cost is negligible for typical doc sets.
- CI must check out full history (`fetch-depth: 0`) for full accuracy.

## Alternatives considered

- **Diff against a PR base (`--base <ref>`)** — flags code changed in the PR
  whose doc was not. Precise for PRs, but needs a base ref and only sees the
  PR's own changes. Commit order gives the same answer for a PR and also works
  on `main` and locally, without extra flags.
- **A pure-Go git implementation (e.g. go-git)** — would comply with ADR 0011
  as a build-time dependency and avoid the subprocess. Rejected for now: it is
  a large dependency tree for four read-only queries, and the binary would
  still need a git repository on disk, i.e. a machine that already has git.
  Worth revisiting if a platform without a git binary needs `git` mode.
- **Commit timestamps instead of commit order** — simpler, but author dates
  are rewritten by rebases and skewed across machines; reachability is exact.
- **A `reviewed: <sha>` front-matter acknowledgement** — would let an accurate
  doc be confirmed without editing prose, but setting it is itself a commit to
  the doc, which already clears the finding. Not worth a new schema field yet.
