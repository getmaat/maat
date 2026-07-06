---
title: Development guide
status: current
summary: How to set up, run, and contribute to Ma'at locally.
related_code:
  - .goreleaser.yaml
  - .github/workflows/release.yml
  - scripts/render-homebrew-formula.sh
  - scripts/install.sh
  - action.yml
  - .github/workflows/maat-check.yml
---

# Development guide

## Prerequisites

- **Go 1.24+** — to build and test Ma'at. There are no other build- or
  run-time dependencies (see [ADR 0005](../decisions/0005-go-rewrite.md)).
- Nothing else. The YAML subset parser is hand-written and vendored in the
  package, so there are no third-party modules to fetch.

## Setup

```bash
git clone <this-repo>
cd maat
go run . --help      # runs straight from the clone
```

To produce a standalone binary you can drop on your `PATH`:

```bash
go build -o maat .
./maat --help
```

## Everyday workflow

The CLI entry point is `main.go` at the repository root; the engine is the
`internal/maat/` package. The layout and responsibilities are described in the
[architecture overview](../architecture/overview.md); read it before making
structural changes.

A normal change loop:

```bash
# 1. make your code change under internal/maat/ (or main.go)
# 2. update the matching doc(s) — see AGENTS.md "update protocol"
go run . sync      # regenerate derived files if docs changed
go run . check     # validate docs (the CI gate)
go test ./...                  # run the tests
```

Both `check` and the tests must be green before you open a pull request.

This repository pins `maat_version: "~> 0.1"` in its own `.maat.yml` (it
dogfoods the feature). Source builds — `go run .`, `go build`, or a
pseudo-versioned binary — are **exempt** from that pin, so it never blocks your
local loop; enforcement applies only to released binaries. See
[ADR 0006](../decisions/0006-distribution-and-versioning.md).

## Releasing

Releases are cut by pushing a semantic-version tag. The
[`release` workflow](../../.github/workflows/release.yml) then runs
[GoReleaser](https://goreleaser.com) (config:
[`.goreleaser.yaml`](../../.goreleaser.yaml)), which cross-compiles the
binaries for linux/darwin/windows on amd64/arm64, builds the archives and
`checksums.txt`, and publishes a GitHub Release.

```bash
# 1. ensure main is green (check + tests) and the tag points at it
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0        # triggers the release workflow
```

The tag is the single source of truth for the version: GoReleaser injects it
via `-ldflags` into `internal/maat`'s `version` variable, so a `v0.1.0` tag
makes the released binary report `maat 0.1.0`. A plain source build with no tag
reports the VCS pseudo-version instead (see `Version()` in `util.go`). Tags
must be `vMAJOR.MINOR.PATCH`; pre-release tags (`v0.2.0-rc1`) are published as
GitHub pre-releases automatically.

Once a tag is pushed, `go install github.com/getmaat/maat@latest` resolves it
through the Go module proxy.

### Homebrew

Ma'at is distributed through a personal tap,
[`getmaat/homebrew-tap`](https://github.com/getmaat/homebrew-tap):

```bash
brew install getmaat/tap/maat
```

This fully-qualified form installs without a prompt: naming the tap on the
command line satisfies Homebrew 6.0+'s tap-trust check. The safeguard only
triggers when the tap is *not* named on the command line — e.g. the short-name
flow (`brew tap getmaat/tap` then `brew install maat`), which fails with
`Refusing to load formula … from untrusted tap` until the user runs
`brew trust getmaat/tap` (stored per-machine in `~/.homebrew/trust.json`).

The tap holds a cross-platform **formula** (`Formula/maat.rb`) that installs the
pre-built binary on both macOS and Linuxbrew. A formula — rather than a Cask — is
deliberate: Casks are macOS-only and trigger a Gatekeeper prompt on our unsigned
binary, whereas a formula installs the binary cleanly on both platforms.

The formula is regenerated on every release by
[`scripts/render-homebrew-formula.sh`](../../scripts/render-homebrew-formula.sh),
which reads the version and `dist/checksums.txt` that GoReleaser produced and
prints the formula to stdout (it is pure — no network, no side effects). The
`release` workflow runs it and pushes the result to the tap. This is wired as a
custom step instead of GoReleaser's deprecated `brews:` block (which now emits a
Cask). Publishing requires a `HOMEBREW_TAP_TOKEN` repository secret on `maat` —
a token with push access to the tap repo; if it is unset, the step is skipped
with a warning and the GitHub Release still succeeds.

To preview or regenerate the formula locally:

```bash
scripts/render-homebrew-formula.sh 0.1.0 dist/checksums.txt
```

A true bare `brew install maat` (no tap) would require acceptance into
homebrew-core, which has a notability bar and ongoing-maintenance obligations —
revisit once the project has traction.

### Install script and GitHub Action

Two more distribution artifacts ride on the same GoReleaser output and are
maintained in this repo (see
[ADR 0006](../decisions/0006-distribution-and-versioning.md)):

- [`scripts/install.sh`](../../scripts/install.sh) — the universal `curl | sh`
  installer. It downloads the `maat_<version>_<os>_<arch>.tar.gz` archive and
  verifies it against `checksums.txt`, so it depends on GoReleaser's archive
  and checksum **naming** staying stable. That same contract is shared by the
  Homebrew renderer, so treat the name template in `.goreleaser.yaml` as an API.
- [`action.yml`](../../action.yml) (composite action) and
  [`.github/workflows/maat-check.yml`](../../.github/workflows/maat-check.yml)
  (reusable workflow) — both install through `install.sh`, so there is one
  install code path to maintain. Consumers pin them to an exact release tag
  (e.g. `getmaat/maat@v0.2.0`); a moving major-version pointer (`@v1`) is
  deferred until 1.0, so it cannot collide with the semver release trigger.

## Coding conventions

- Standard library only — no third-party runtime dependencies.
- Keep generators pure (no I/O); all disk writes live in `sync.go`.
- Match the existing module boundaries; see
  [conventions](../meta/conventions.md).
