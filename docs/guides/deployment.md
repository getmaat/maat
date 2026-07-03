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
[ADR 0005](../decisions/0005-go-rewrite.md)). Obtain it either way:

```bash
# Build from source (requires Go 1.24+):
go build -o maat .
# …then move ./maat onto your PATH.

# Or run without installing, from a clone of this repo:
go run . <command>
```

A prebuilt binary can be committed to the target repo or fetched in CI; because
it is statically linked, no interpreter or package manager is required on the
machine that runs it.

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

The generated [`.github/workflows/maat.yml`](../../.github/workflows/maat.yml)
builds the binary and runs `maat check` on every pull request. For non-GitHub
CI, run the same command:

```bash
maat check --format text
```

Make the job **required** so documentation drift blocks a merge, mirroring how
a failing test does.

## 5. Establish the habit

From here, the [update protocol](../../AGENTS.md) does the work: every change
that touches code updates its docs in the same PR, `maat sync` regenerates
derived files, and `maat check` keeps everyone honest.
