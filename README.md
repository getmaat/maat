# Ma'at

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset=".github/assets/maat-logo.webp">
    <source media="(prefers-color-scheme: light)" srcset=".github/assets/maat-logo-light.webp">
    <img src=".github/assets/maat-logo-light.webp" alt="Ma'at logo" width="420">
  </picture>
</p>

<p align="center">
  <strong>Documentation-as-code for humans and AI agents.</strong>
</p>

<p align="center">
  Keep your docs, source code, and AI-agent instructions in sync — within a repository and across repositories. Fail CI when they drift.
</p>

<p align="center">
  <a href="https://github.com/getmaat/maat/actions/workflows/maat.yml">
    <img src="https://github.com/getmaat/maat/actions/workflows/maat.yml/badge.svg" alt="CI">
  </a>
  <a href="https://github.com/getmaat/maat/releases/latest">
    <img src="https://img.shields.io/github/v/release/getmaat/maat" alt="Release">
  </a>
  <a href="LICENSE">
    <img src="https://img.shields.io/github/license/getmaat/maat" alt="License">
  </a>
</p>

<!--
<p align="center">
  <img src=".github/assets/banner-placeholder.png" alt="Ma'at keeps documentation, source code, and AI-agent instructions in sync">
</p>
-->

---

## Why Ma'at?

Modern repositories have two audiences:

- **Humans** need accurate architecture notes, guides, ADRs, and references.
- **AI coding agents** need setup steps, project rules, conventions, and task context.

But that knowledge usually gets scattered across:

- `docs/`
- `AGENTS.md`
- `CLAUDE.md`
- `.github/copilot-instructions.md`
- `.cursor/rules/...`
- `.windsurf/rules/...`
- `GEMINI.md`
- other agent-specific files

Maintaining all of that by hand guarantees drift.

**Ma'at makes one source of truth explicit, generates the derived files, and checks everything in CI.**

---

## Fast start

Install Ma'at:

```bash
# Homebrew
brew install getmaat/tap/maat

# Install script, no Go toolchain required
curl -sSf https://raw.githubusercontent.com/getmaat/maat/main/scripts/install.sh | sh

# Go
go install github.com/getmaat/maat@latest
```

Scaffold your repository:

```bash
maat init . --name "My Project" --summary "What it does"
```

Validate it:

```bash
maat check
```

![maat init scaffolding a repository, then maat check validating it](.github/assets/demo.gif)

That gives your repo a documentation-as-code structure, generated AI-agent adapter files, and a CI gate that treats documentation drift like a failing test.

---

## What Ma'at gives you

| Problem | Ma'at's answer |
| --- | --- |
| Docs rot after code changes | Docs declare the source files they cover via `related_code`; `maat check` uses git history to flag a doc whose code changed after it. |
| Every AI agent wants a different instruction file | Keep `AGENTS.md` canonical; generate agent-specific adapters. |
| New contributors do not know where durable knowledge lives | Scaffold a predictable `docs/` tree with guides, architecture docs, ADRs, references, and indexes. |
| Generated files drift from source | `maat sync` regenerates them; `maat check` verifies them. |
| Teams forget to update docs | CI fails the pull request, just like a test failure. |
| An agent in one repo knows nothing about the APIs of the others | Docs declare the contracts they `provide` and `consume`; every repo publishes a machine-readable manifest. |
| A provider changes an API and consumers find out in production | Consumers lock the contracts they use; `maat check` fails on an incompatible version, and a scheduled job reports provider changes. |

---

## The core workflow

```text
edit code
   ↓
update docs in the same change
   ↓
run maat sync
   ↓
run maat check
   ↓
open PR
   ↓
CI verifies docs, indexes, and agent files are still in sync
```

A change is not complete until its documentation is updated in the same pull request.

---

## What gets scaffolded?

```text
AGENTS.md                        # canonical cross-agent instructions
docs/
  index.md                       # human documentation entry point
  llms.txt                       # machine-readable documentation index
  architecture/
  decisions/
  guides/
  reference/
  meta/
.maat/
  manifest.json                  # generated: the contracts this repo provides and consumes
  skills/                        # generated agent skills (e.g. retrospect)
  templates/                     # module doc template (ADR template lives at docs/decisions/_template.md)
.maat.yml                        # Ma'at configuration
.github/workflows/maat.yml       # CI gate

CLAUDE.md
GEMINI.md
.hermes.md
.github/copilot-instructions.md
.cursor/rules/maat.mdc
.windsurf/rules/maat.md          # generated agent adapters
```

`maat init` is safe to re-run. It creates the Ma'at structure without overwriting existing project-owned files.

If your repository already has an `AGENTS.md`, Ma'at preserves your prose and inserts its maintenance contract into a managed block. Your hand-written content remains yours; Ma'at only updates the managed block.

---

## How it works

```text
                         .maat.yml
                              │
                              ▼
   docs/*.md  ──scan──▶  DocsModel  ──┬── generate ──▶ llms.txt, adapters, index, manifest
  AGENTS.md                           │
                                      └── validate ──▶ pass/fail, the CI gate
                                              ▲
                     .maat/contracts.lock.json  (other repos' contracts, fetched on demand)
```

- **`docs/` + `AGENTS.md`** are the source of truth — hand-written, reviewed in PRs, and versioned with the code.
- **`docs/llms.txt`**, the per-agent adapter files, the navigation in `docs/index.md`, and the contract manifest are generated from that source.
- **`maat check`** verifies front matter, links, `related_code`, staleness, contracts, and generated-file drift — fully offline and deterministic.
- **CI fails** when docs or generated files fall behind the code.

---

## Contracts across repositories

Real systems span repositories: a service documents an API that three others call. Ma'at lets docs declare those edges, so they are reviewed like any other change and checked in CI.

A doc that describes an interface declares what it **provides**; a doc whose subject depends on one declares what it **consumes**:

```yaml
---
title: Invoice API
status: current
related_code:
  - internal/billing/api.go
provides:
  - id: invoice-api
    kind: http-api
    version: 2.1.0
    spec: api/openapi.yaml       # a spec change marks this doc stale
consumes:
  - ref: acme/ledger#ledger-events
    version: "~> 1.4"
---
```

- `maat sync` collects every edge into **`.maat/manifest.json`**, a fixed path any tool or repository can read, and lists them in `llms.txt` so agents see the interfaces up front.
- Refs within the repository are checked directly: they must resolve, and versions must satisfy the constraint.
- For refs to **other repositories**, list where their manifests live and lock them:

  ```yaml
  # .maat.yml
  repo: acme/billing
  contracts:
    sources:
      acme/ledger:                # GitHub by default; URL templates and local paths also work
  ```

  ```bash
  maat contracts update           # fetch the consumed contracts into .maat/contracts.lock.json
  ```

  `maat check` validates cross-repo refs against that committed lockfile, offline. A scheduled CI job running `maat contracts update --check` fails when a provider has changed since — so a breaking change arrives as a reviewable lockfile diff, not as a red build on someone's unrelated pull request.

Details: [contract fields](docs/reference/frontmatter.md#provides-and-consumes), [manifest format](docs/reference/manifest.md), [lockfile and sources](docs/reference/contracts-lock.md), and the design in [ADR 0013](docs/decisions/0013-contract-edges-and-manifest.md) and [ADR 0014](docs/decisions/0014-cross-repo-contracts-via-lockfile.md).

---

## CLI

A single static binary with zero runtime dependencies. It uses `git` for staleness when available, and touches the network only in `maat contracts update`.

```bash
maat init .                # scaffold docs/, AGENTS.md, config, CI, .maat/templates, and adapters
maat sync                  # regenerate llms.txt, adapters, index navigation, and the manifest
maat check                 # validate docs and fail on drift
maat contracts update      # lock the contracts consumed from other repositories
```

Running from a clone without installing:

```bash
go build -o maat .
go run . <command>
```

| Command | Does |
| --- | --- |
| `init` | Stamps the framework into a repository. Safe to re-run; never overwrites existing files. |
| `sync` | Regenerates every derived file from the docs tree and `AGENTS.md`. |
| `check` | Validates front matter, links, `related_code`, staleness, contracts, and drift. Offline. |
| `contracts update` | Fetches the manifests of consumed repositories into the lockfile; `--check` only reports. |

Full reference: [`docs/reference/cli.md`](docs/reference/cli.md).

---

## GitHub Actions

Use Ma'at in CI to block documentation drift on pull requests.

```yaml
name: maat

on:
  pull_request:
  push:
    branches: [main]

jobs:
  maat:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0   # full history, so staleness can compare commits
      - uses: getmaat/maat@v0.9.0
        with:
          version: "0.9.0"
```

Make this job required if you want documentation drift to block merges.

If your docs consume contracts from other repositories, add a scheduled job that checks the lockfile is still current. The reusable workflow has it built in:

```yaml
on:
  schedule: [{cron: "17 6 * * *"}]
jobs:
  contracts:
    uses: getmaat/maat/.github/workflows/maat-check.yml@v0.9.0
    with:
      contracts_update_check: true
```

See the [deployment guide](docs/guides/deployment.md) for tokens, private repositories, and non-GitHub CI.

---

## Who is this for?

Ma'at is useful if your project:

- uses AI coding agents such as GitHub Copilot, Claude Code, Codex, Cursor, Windsurf, opencode, Hermes, Gemini CLI, or similar tools;
- has docs that should stay versioned with the code;
- wants one canonical source for contributor and agent instructions;
- wants CI to catch stale documentation;
- is split across several repositories whose APIs depend on each other;
- wants a lightweight convention instead of a hosted docs platform.

Ma'at is **not** a documentation website generator.

It is a repository-local convention and CLI for keeping docs, source code, and AI-agent instructions aligned.

---

## Documentation

This repository dogfoods Ma'at.

- Human entry point: [`docs/index.md`](docs/index.md)
- Agent entry point: [`docs/llms.txt`](docs/llms.txt)
- Design rationale: [`docs/decisions/`](docs/decisions/)
- Adoption guide: [`docs/guides/deployment.md`](docs/guides/deployment.md)
- Contributor protocol: [`AGENTS.md`](AGENTS.md)

---

## The update protocol

A change is not complete until its docs are updated in the same change.

See [`AGENTS.md`](AGENTS.md) for the full protocol every contributor — human or agent — follows.

---

## Contributing

Bug reports, feature requests, and pull requests are welcome.

Before opening a PR:

```bash
go test ./...
go run . sync
go run . check
```

See [`CONTRIBUTING.md`](CONTRIBUTING.md) for development setup.

---

## License

Apache-2.0 — see [`LICENSE`](LICENSE).
