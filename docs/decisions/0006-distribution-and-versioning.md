---
title: 0006 Distribution and version pinning
status: current
summary: Ma'at is adopted through a CI gate as the authority, a universal curl|sh installer plus published Action/reusable workflow, and an in-config maat_version constraint the released binary self-enforces.
---

# 0006. Distribution and version pinning

- **Status:** accepted
- **Date:** 2026-07-03
- **Relates to:** [0005 Rewrite the CLI in Go, distribute a single static binary](0005-go-rewrite.md)

## Context

[ADR 0005](0005-go-rewrite.md) made *running* Ma'at frictionless: one static
binary, no interpreter. It did not answer how a team *obtains a specific
version* and *guarantees everyone — including CI — uses a compatible one*.

The existing distribution channels all assume a toolchain the target repo may
not have:

- `go install` requires Go.
- The Homebrew tap requires macOS/Linuxbrew.
- Building from source requires Go 1.24+.

A repository written in TypeScript, Ruby, or C++, running CI on a stock Ubuntu
image, has none of these. That is precisely the audience ADR 0005 promised to
serve, so a **language-agnostic install primitive** is the missing piece.

Separately, nothing pins a *version*. If one contributor runs an old `maat` and
another runs a new one, they can generate different `llms.txt`/adapter output
and thrash the drift check, or disagree about what `check` enforces. Mature
tooling solves this by letting a repo declare the version it expects and having
the tool enforce it: Terraform's `required_version`, Corepack's
`packageManager`, `pre-commit`'s pinned `rev`, asdf/mise's `.tool-versions`.

The load-bearing principle that shapes the whole design: **CI is the authority,
local installs are an optional accelerant.** The gate that actually protects
`main` is `maat check` running in CI. No developer's machine is required to be
correctly set up for the guarantee to hold — a local binary only gives faster
feedback before the push.

## Decision

Adopt a three-layer distribution and version-pinning model.

### 1. CI is the enforcing authority

The canonical way to adopt Ma'at is a CI job that installs a pinned binary and
runs `maat check`. We ship two ready-made entry points so a repo needs one
line, not a hand-rolled job:

- A **composite GitHub Action** (`action.yml`) — `uses: getmaat/maat@v0.2.0`
  with an optional `version` input.
- A **reusable workflow** (`.github/workflows/maat-check.yml`) callable via
  `workflow_call`, so an org maintains the runner/checkout/flags in one place
  and every caller inherits changes.

Both default to emitting GitHub annotations (`--format github`) so findings
surface inline on the PR.

### 2. A universal `curl | sh` installer

`scripts/install.sh` is the language-agnostic primitive underneath everything
else:

```sh
curl -sSf https://raw.githubusercontent.com/getmaat/maat/main/scripts/install.sh | sh
```

It is POSIX `sh` (no bashisms), detects OS/arch, downloads the matching
GoReleaser archive, **verifies the sha256 against `checksums.txt`** by default,
and installs to a sensible bin dir. `MAAT_VERSION` pins the exact release;
`MAAT_INSTALL_DIR` and `MAAT_NO_VERIFY` cover the escape hatches. It needs only
tools present on a stock macOS or Linux CI image (curl/wget, tar, sha256sum/
shasum). The Action installs through this same script, so there is one code
path to maintain.

### 3. An in-config version pin the binary self-enforces

A repo declares the version it expects in `.maat.yml`:

```yaml
maat_version: "~> 0.1"
```

The constraint grammar is a small, Terraform-style subset implemented in
`internal/maat/semver.go` (dependency-free, like the YAML subset, per the
single-binary promise): `~>` (pessimistic), `>=`, `>`, `<=`, `<`, `=`, and a
bare version meaning `=`, combined with commas as AND.

`maat sync` and `maat check` evaluate the constraint at startup. A **released**
binary that does not satisfy it exits `2` with an upgrade hint. **Development
builds are exempt** — a source build, `go run`, or a VCS pseudo-version is
never blocked, so contributors hacking on a repo (or on Ma'at itself) are not
locked out by a pin they are mid-way through changing. A malformed constraint
fails config validation for *everyone*, dev build or not, so typos surface
immediately.

This mirrors the "declare in config, tool enforces, fail fast" model of
`required_version`/`packageManager` rather than a separate lockfile, keeping the
single source of truth in the file the team already edits.

## Consequences

- **Any repo, any CI, no toolchain** can install a pinned Ma'at in one line —
  closing the gap ADR 0005 left between "runs frictionlessly" and "obtainable
  frictionlessly."
- **Version skew is caught, not debugged.** A repo pins once; released binaries
  that drift out of range refuse to run with a clear message instead of
  silently producing different output.
- **Contributors are never blocked** by the pin, because dev builds are exempt.
  The guarantee targets released binaries, which are what teams and CI actually
  run.
- The scaffolded CI workflow (`maat init`) now installs via the script instead
  of building from a vendored Go source tree — correct for the non-Go repos
  that are the majority of the audience. When `init` runs from a released
  binary it stamps that exact version into the workflow for reproducibility;
  from a dev build it leaves the version empty (track latest).
- We take on a small **semver subset** to maintain. Accepted for the same
  reason as the YAML subset: it is small, and a runtime dependency would
  compromise the single-binary guarantee.
- The installer depends on the **GoReleaser archive/checksum naming** staying
  stable (`maat_<version>_<os>_<arch>.tar.gz` + `checksums.txt`). That contract
  is now shared by three consumers — the install script, the Homebrew formula
  renderer, and the Action — so changing it is a coordinated change.

## Alternatives considered

- **A committed `./maatw` wrapper** (Gradle/Maven `gradlew`/`mvnw` style) that
  downloads the pinned binary on first use. Strong reproducibility and the most
  self-contained option, but it drops a binary-fetching script into every
  adopting repo and adds a `.maat-version` file. Deferred as a possible phase-2
  addition; the config pin + installer covers the same need with less intrusion.
- **A separate `.maat-version` lockfile** (asdf/mise `.tool-versions` style).
  Rejected as the primary mechanism: it splits the source of truth from the
  config the team already edits. `maat_version` in `.maat.yml` is one file.
- **No enforcement, documentation only** ("please run version X"). Rejected:
  the drift check actively punishes version skew, so an unenforced convention
  would generate confusing failures rather than prevent them.
- **A full semver library dependency.** Rejected to preserve the zero-runtime-
  dependency, single-static-binary guarantee of ADR 0005; the needed subset is
  small enough to own.
