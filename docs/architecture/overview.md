---
title: Architecture Overview
status: current
summary: How Ma'at is built — the CLI pipeline, the docs model, and the generate/validate split.
---

# Architecture overview

Ma'at is a small, single-binary Go CLI plus a documented convention for
how a repository's `docs/` tree is structured. There is no server, no database,
and no runtime dependency — it compiles to one static executable (with the
scaffold templates embedded via `//go:embed`) so the tool can be dropped into
any repository and run anywhere, regardless of the project's language. A
handful of Go module dependencies compile *into* that binary (terminal
styling, an interactive prompt) — see
[ADR 0011](../decisions/0011-build-time-go-dependencies.md) for why that's
compatible with "no runtime dependency."

## The three commands

Everything the user does goes through one of three verbs:

```
maat init    →  stamp the docs/ scaffold + AGENTS.md + config + CI into a repo
maat sync    →  regenerate derived files (llms.txt, adapters, index nav)
maat check   →  validate the docs set; exit non-zero on problems (the CI gate)
```

## Data flow

```
                         .maat.yml
                              │  (config.Load)
                              ▼
   docs/*.md  ──scan──▶  DocsModel  ──┬── generate ──▶ llms.txt, adapters, index nav
  (frontmatter)         (model.go)    │                 (sync writes them)
                                      └── validate ──▶ Findings ──▶ exit code
                                          (check.go)              (check reports them)
```

The **single source of truth** is the `docs/` tree plus the root `AGENTS.md`.
Every other agent-facing file (`CLAUDE.md`, `.github/copilot-instructions.md`,
`.cursor/rules/maat.mdc`, `docs/llms.txt`, …) is *derived* by `sync` and
*verified* by `check`. This is the core invariant: **derived files never hold
original information**, so they can always be regenerated and can never
silently disagree with the source.

## Why sync and check share one function

`sync` and `check` both call `expectedArtifacts()` (in `sync.go`), which returns
an ordered `{path: desired_content}` map. `sync` writes it; `check` compares the
map against what is on disk and reports "drift" for any mismatch. Because both
commands derive their notion of "correct" from the same function, the sequence
*sync then check* can never fail on drift — drift can only appear from a hand
edit to a generated file or a forgotten `sync`.

## Components

| Component | File | Responsibility |
|-----------|------|----------------|
| CLI / arg parsing | `internal/maat/cli.go` | Parse args, dispatch to commands, format output |
| Docs scanner/model | `internal/maat/model.go` | Walk `docs/`, parse front-matter, index documents |
| Generators | `internal/maat/generate.go` | Render `llms.txt`, adapter files, index navigation |
| Sync command | `internal/maat/sync.go` | Compute + write derived artifacts |
| Check command | `internal/maat/check.go` | Validate front-matter, links, code refs, drift |
| Staleness probes | `internal/maat/staleness.go` | Decide whether a doc lags its code (git history, mtime fallback) |
| Contracts | `internal/maat/contracts.go`, `lockfile.go` | Validate `provides`/`consumes` edges (cross-repo ones against the lockfile), render the manifest |
| Contracts update | `internal/maat/contracts_update.go`, `fetch.go` | Fetch provider manifests into the lockfile — the only networked code |
| Init + scaffold | `internal/maat/scaffold.go` | Stamp the scaffold into a repo (templates embedded via `//go:embed`) |
| Config | `internal/maat/config.go` | Load `.maat.yml`, defaults, adapter registry |
| Front-matter I/O | `internal/maat/frontmatter.go` | Split/join the `---` YAML block in Markdown |
| YAML subset | `internal/maat/yaml.go` | Hand-rolled, read-only YAML subset parser |
| Terminal presentation | `internal/maat/tty.go`, `style.go`, `wizard.go` | TTY/color detection, Lip Gloss styling, the Huh `init` wizard |
| Entry point | `main.go` | `main()` — calls into `internal/maat` |

## Module index

- [YAML & front-matter](modules/yaml-frontmatter.md) — the hand-rolled
  parsing layer everything else is built on.
- [Model & generators](modules/model-generate.md) — scanning docs and
  rendering derived artifacts.
- [Check engine](modules/check.md) — the validation rules behind the CI gate.
- [Contracts update](modules/contracts-update.md) — fetching provider
  manifests into the lockfile; the only networked code.
- [Init & scaffold](modules/scaffold.md) — what `maat init` stamps out, the
  embedded templates, and version-dependent values.
- [Terminal presentation](modules/presentation.md) — TTY/color detection,
  styled output, and the interactive `init` wizard.

## Key design decisions

The *why* behind this shape is recorded as ADRs:

- [0005 Rewrite the CLI in Go, distribute a single static binary](../decisions/0005-go-rewrite.md)
- [0003 AGENTS.md as source of truth with generated adapters](../decisions/0003-agents-md-source-of-truth.md)
- [0012 Detect staleness from git history](../decisions/0012-git-aware-staleness.md) (supersedes 0004)
- [0013 Typed contract edges in front matter and a generated manifest](../decisions/0013-contract-edges-and-manifest.md)
- [0011 Build-time Go dependencies are permitted; runtime dependencies are not](../decisions/0011-build-time-go-dependencies.md)
