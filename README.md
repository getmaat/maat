# Ma'at

[![CI](https://github.com/getmaat/maat/actions/workflows/maat.yml/badge.svg)](https://github.com/getmaat/maat/actions/workflows/maat.yml)
[![Release](https://img.shields.io/github/v/release/getmaat/maat)](https://github.com/getmaat/maat/releases/latest)
[![License](https://img.shields.io/github/license/getmaat/maat)](LICENSE)

**Documentation-as-code for humans _and_ AI agents.**

Ma'at keeps three things in lockstep — a repository's `docs/` tree, its
cross-agent instruction files, and its source code — so that documentation can
be maintained interchangeably by AI coding agents, human developers, and CI.

Whatever agent harness a developer uses — GitHub Copilot, Claude Code, Codex,
Cursor, Windsurf, opencode, Hermes, Gemini CLI, and others — Ma'at gives it a
single, discoverable source of truth and a protocol for keeping the docs
current as the code changes.

![`maat init` scaffolding a repository, then `maat check` validating it](.github/assets/demo.gif)

---

## Why

Two problems, one solution:

1. **Docs rot.** They live apart from the code, are updated late (if ever), and
   silently drift until they mislead. Ma'at treats docs as part of the diff
   and gives CI a way to *fail the build* when they fall behind.
2. **Every agent looks somewhere different.** Claude reads `CLAUDE.md`, Copilot
   reads `.github/copilot-instructions.md`, Cursor reads `.cursor/rules/…`, and
   a growing set read `AGENTS.md`. Maintaining these by hand guarantees they
   diverge. Ma'at makes **`AGENTS.md` the single source of truth** and
   *generates* every other agent's file from it, verifying they never drift.

## Install

```bash
# Homebrew (macOS + Linuxbrew)
brew install getmaat/tap/maat

# Install script (no Go toolchain required)
curl -sSf https://raw.githubusercontent.com/getmaat/maat/main/scripts/install.sh | sh

# Go
go install github.com/getmaat/maat@latest
```

Or drop the [`getmaat/maat`](action.yml) GitHub Action into CI to run `maat
check` on every pull request — see
[`docs/guides/deployment.md`](docs/guides/deployment.md).

## How it works

```
                         .maat.yml
                              │
                              ▼
   docs/*.md  ──scan──▶  DocsModel  ──┬── generate ──▶ llms.txt, adapters, index
  (frontmatter)                       │
                                      └── validate ──▶ pass/fail  (the CI gate)
```

- **`docs/` + `AGENTS.md`** are the source of truth — hand-written, reviewed in
  PRs, versioned with the code.
- **`docs/llms.txt`**, the per-agent adapter files, and the navigation in
  `docs/index.md` are **generated** from that source and **verified** in CI, so
  they can never silently disagree.
- Docs declare the source files they describe via `related_code` front-matter,
  which lets CI detect when code changed but its doc didn't.

## The CLI

A single static binary with zero runtime dependencies.

```bash
maat init .     # scaffold docs/, AGENTS.md, config, CI, adapters
maat sync       # regenerate llms.txt + adapters + index nav
maat check      # validate docs; non-zero exit fails CI
```

Running from a clone without installing:

```bash
go build -o maat .   # produce the binary
go run . <command>   # or run without building
```

| Command | Does |
|---------|------|
| `init` | Stamps the framework into a repository (safe to re-run) |
| `sync` | Regenerates every derived file from the docs tree |
| `check` | Validates front-matter, links, `related_code`, staleness, and drift |

## What gets scaffolded

```
AGENTS.md                        ← canonical, cross-agent instructions
docs/
  index.md                       ← human entry point (managed nav block)
  llms.txt                       ← machine-readable index (generated)
  architecture/  decisions/  guides/  reference/  meta/
templates/                       ← ADR + module templates
.maat.yml                        ← configuration
.github/workflows/maat.yml       ← CI gate
CLAUDE.md  .hermes.md  GEMINI.md  .github/copilot-instructions.md
.cursor/rules/maat.mdc  .windsurf/rules/maat.md   ← generated adapters
```

## Documentation

This repository dogfoods itself — its own docs are built with Ma'at. Start at
[`docs/llms.txt`](docs/llms.txt) (for agents) or [`docs/index.md`](docs/index.md)
(for humans). Design rationale lives in
[`docs/decisions/`](docs/decisions/).

## The update protocol

A change is not complete until its docs are updated in the same change. See
[`AGENTS.md`](AGENTS.md) for the full protocol every contributor — human or
agent — follows.

## Contributing

Bug reports, feature requests, and pull requests are welcome — see
[`CONTRIBUTING.md`](CONTRIBUTING.md) for dev setup and the docs-update
protocol every change follows.

## License

Apache-2.0 — see [LICENSE](LICENSE).
