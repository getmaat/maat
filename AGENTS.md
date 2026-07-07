# Ma'at

<!--
This is the single source of truth for AI coding agents working in this
repository. It follows the AGENTS.md open standard (https://agents.md) and is
read natively by Codex, Cursor, Gemini CLI, Jules, Factory, Aider, goose,
opencode, Zed, GitHub Copilot's coding agent, Windsurf, Devin, Hermes, and
others. Agents that use a different filename are pointed here by generated
adapter files (CLAUDE.md, .github/copilot-instructions.md, .cursor/rules/…).

Keep this file concise and stable. Detailed, changing knowledge belongs in
docs/ — this file tells agents how to FIND and MAINTAIN those docs.
-->

## Project overview

Ma'at is a **documentation-as-code framework** that keeps three things in
lockstep: a repository's `docs/` tree, its cross-agent instruction files, and
its source code. It is designed so that documentation can be maintained
interchangeably by AI coding agents, human developers, and CI — and so that
**any** agent harness (Claude Code, Copilot, Codex, Cursor, Windsurf, Hermes,
opencode, and others) discovers and updates the same docs.

Ma'at ships a small, **zero-dependency Go CLI** (`maat`) — distributed as
a single static binary — with three verbs — `init`, `sync`, `check` — and a
documented convention for how the `docs/` tree is structured and kept current.

This repository **dogfoods itself**: the docs you are reading were scaffolded
by `maat init` and are validated by `maat check` in CI.

## Where the documentation lives

All durable knowledge lives in [`docs/`](docs/). It is documentation-as-code:
versioned with the source, reviewed in pull requests, and treated as a
first-class part of every change.

- **Start here:** [`docs/llms.txt`](docs/llms.txt) — a machine-readable index
  of every document. Read it first to locate what a task needs.
- **Human entry point:** [`docs/index.md`](docs/index.md).
- Organized into:
  - [`docs/architecture/`](docs/architecture/) — how the system is built.
  - [`docs/decisions/`](docs/decisions/) — Architecture Decision Records; the
    *why*, append-only.
  - [`docs/guides/`](docs/guides/) — how to develop, test, deploy, troubleshoot.
  - [`docs/reference/`](docs/reference/) — factual surface: CLI, config,
    front-matter schema.
  - [`docs/meta/`](docs/meta/) — conventions and glossary for the docs.

## Before you change code

1. Read [`docs/llms.txt`](docs/llms.txt) and open the documents relevant to
   your task.
2. Check [`docs/decisions/`](docs/decisions/) for any ADR that constrains the
   area you are about to touch. Do not silently contradict an accepted
   decision — if one should change, write a new ADR that supersedes it.

<!--
The Documentation update protocol and the front-matter schema are Ma'at
framework invariants (ADR 0009). They are maintained for every repo in the
generated managed block below (the same block that carries the skills list),
so they are not hand-written here. `maat sync` splices them in; edit the
generators, not the block. Full front-matter schema: docs/reference/frontmatter.md.
-->

## Setup, build, and test commands

Ma'at is a single static binary with **no runtime dependencies**. Building it
requires Go 1.24+.

```bash
# Run the CLI from a clone (no install needed):
go run . --help

# Build a standalone binary:
go build -o maat .

# Validate the docs (this is what CI runs):
go run . check

# Regenerate derived files after editing docs:
go run . sync

# Run the test suite:
go test ./...

# Lint (see docs/guides/testing.md):
golangci-lint run ./...
```

Before declaring any change done: run `go run . check`, `go test ./...`, and
`golangci-lint run ./...`, and make sure all three are green.

## Human approval

Agents may draft documentation changes, but a human reviews them in the pull
request alongside the code. CI (`maat check`) enforces that docs were kept
in sync; the reviewer confirms they are *correct*.

<!-- maat:begin (generated — edit the source docs, not this block) -->
## Documentation update protocol

**A change is not complete until its documentation is updated in the same
change.** Treat docs edits as part of the diff, never a follow-up.

When you modify code, update docs as follows:

| If you… | Then update… |
|---|---|
| Change how a module works or how modules relate | the module's page in `docs/architecture/` |
| Make a non-obvious, hard-to-reverse choice | add a new ADR in `docs/decisions/` (copy `_template.md`) |
| Change build/test/deploy/run steps | the relevant `docs/guides/` page |
| Add/rename/remove a CLI flag, config key, or front-matter field | `docs/reference/` |
| Add or move a source file a doc's `related_code` points at | that doc's `related_code` front-matter |

Then regenerate derived indexes and adapter files, and validate before
committing:

```bash
maat sync      # regenerate llms.txt, index nav, adapters, and this block
maat check     # fails on stale/broken/missing/drifted docs
```

### Front-matter every doc carries

Each Markdown file in `docs/` begins with a front-matter block. The
`related_code` list is what lets tooling detect when code drifts from docs:

```markdown
---
title: Human-readable title
status: current            # current | draft | deprecated
summary: One-line description used in indexes.
related_code:              # source paths this doc describes (optional)
  - src/module/thing.ext
---
```

## Skills (reusable procedures)

Ma'at ships step-by-step procedures for recurring documentation tasks
under `.maat/skills/`. When a task matches one, read the skill file
and follow it.

- [`retrospect`](.maat/skills/retrospect/SKILL.md) — Retrofit Ma'at documentation onto an existing repository: inventory gaps, interview the developer, derive documentation and retrospective ADRs.

These files are generated — `maat sync` regenerates them, and hand-edits
are overwritten. Team-authored skills may live alongside them and are
never touched.
<!-- maat:end -->
