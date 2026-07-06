---
title: Adopting Ma'at in a repository
status: current
summary: How to add Ma'at to an existing project and wire it into CI.
---

# Adopting Ma'at in a repository

Ma'at is not a service you deploy; it is a tool and a convention you adopt
into a repository. This guide covers rolling it out.

## 1. Get the tool

Ma'at is a single static binary with no runtime dependencies (see
[ADR 0005](../decisions/0005-go-rewrite.md)), so it drops onto any machine or CI
runner regardless of the language your project is written in. Pick whichever
channel fits:

```bash
# Universal installer (macOS/Linux, no toolchain required):
curl -sSf https://raw.githubusercontent.com/getmaat/maat/main/scripts/install.sh | sh

# Pin an exact version (recommended for CI — see step 4):
curl -sSf https://raw.githubusercontent.com/getmaat/maat/main/scripts/install.sh | MAAT_VERSION=0.2.0 sh

# Homebrew:
brew install getmaat/tap/maat

# Go toolchain:
go install github.com/getmaat/maat@latest
```

The installer detects your OS/arch, downloads the matching release archive,
**verifies its sha256 checksum**, and installs to `/usr/local/bin` (or
`$HOME/.local/bin` if that is not writable; override with `MAAT_INSTALL_DIR`).
Because the binary is statically linked, no interpreter or package manager is
required on the machine that runs it.

## 2. Scaffold the docs

```bash
maat init . --name "My Project" --summary "What it does."
```

This creates `AGENTS.md`, the `docs/` tree, `templates/`, `.maat.yml`, the
CI workflow, and generates `llms.txt` and the agent adapter files. Existing
files are not overwritten.

## 3. Fill in the starters

Edit `AGENTS.md`'s project overview and the scaffolded docs. Delete adapters
you do not need from `.maat.yml` and re-run `maat sync`. Commit
everything, including the generated files (agents read them from a fresh
clone).

## 4. Wire up CI

CI is the authority: a required `maat check` job is what actually blocks a merge
on documentation drift (see
[ADR 0006](../decisions/0006-distribution-and-versioning.md)). The generated
[`.github/workflows/maat.yml`](../../.github/workflows/maat.yml) installs the
binary via the universal script and runs `maat check` on every pull request.

On GitHub, the shortest wiring is the published composite action:

```yaml
- uses: getmaat/maat@v0.2.0
  with:
    version: "0.2.0"   # optional; omit to track the latest release
```

An org with many repos can instead call the reusable workflow so the runner,
checkout, and flags live in one place:

```yaml
jobs:
  maat:
    uses: getmaat/maat/.github/workflows/maat-check.yml@v0.2.0
    with:
      maat_version: "0.2.0"
```

For non-GitHub CI, install the binary and run the same command:

```bash
curl -sSf https://raw.githubusercontent.com/getmaat/maat/main/scripts/install.sh | MAAT_VERSION=0.2.0 sh
maat check --format text
```

Make the job **required** so documentation drift blocks a merge, mirroring how
a failing test does.

## 5. Pin the version

To keep every contributor and CI on a compatible tool, declare the version your
repo expects in `.maat.yml`:

```yaml
maat_version: "~> 0.1"
```

A released binary that does not satisfy the constraint refuses to run (exit
`2`) with an upgrade hint; source builds are exempt. Keep this in step with the
`MAAT_VERSION` your CI installs. The full grammar is in the
[configuration reference](../reference/configuration.md#maat_version).

## 6. Establish the habit

From here, the [update protocol](../../AGENTS.md) does the work: every change
that touches code updates its docs in the same PR, `maat sync` regenerates
derived files, and `maat check` keeps everyone honest.
