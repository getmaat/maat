# {{PROJECT}}

<!--
This is the single source of truth for AI coding agents working in this
repository. It follows the AGENTS.md open standard (https://agents.md) and is
read natively by Codex, Cursor, Gemini CLI, Jules, Factory, Aider, goose,
opencode, Zed, GitHub Copilot's coding agent, Windsurf, Devin, Hermes, and
others. Agents that use a different filename are pointed here by generated
adapter files (CLAUDE.md, .github/copilot-instructions.md, etc.).

Keep this file concise and stable. Detailed, changing knowledge belongs in
docs/ — this file tells agents how to FIND and MAINTAIN those docs.
-->

## Project overview

{{SUMMARY}}

<!-- Replace with 2-4 sentences: what this project is, who uses it, and the
one or two things an agent most needs to know before touching the code. -->

## Where the documentation lives

All durable knowledge lives in [`docs/`](docs/). It is documentation-as-code:
versioned with the source, reviewed in pull requests, and treated as a
first-class part of every change.

- **Start here:** [`docs/llms.txt`](docs/llms.txt) — a machine-readable index
  of every document. Read it first to locate what a task needs.
- **Human entry point:** [`docs/index.md`](docs/index.md).
- Documentation is organized into:
  - `docs/architecture/` — how the system is built (the *what*).
  - `docs/decisions/` — Architecture Decision Records; the *why*, append-only.
  - `docs/guides/` — how to develop, test, deploy, troubleshoot (the *how*).
  - `docs/reference/` — factual surface: configuration, environment, API.
  - `docs/meta/` — conventions and glossary for the docs themselves.

## Before you change code

1. Read [`docs/llms.txt`](docs/llms.txt) and open the documents relevant to
   your task.
2. Check `docs/decisions/` for any ADR that constrains the area you are about
   to touch. Do not silently contradict an accepted decision — if you believe
   one should change, write a new ADR that supersedes it.

## Documentation update protocol

**A change is not complete until its documentation is updated in the same
change.** Treat docs edits as part of the diff, never a follow-up.

When you modify code, update docs as follows:

| If you… | Then update… |
|---|---|
| Change how a module works or relate to each other | the module's page in `docs/architecture/` |
| Make a non-obvious, hard-to-reverse choice | add a new ADR in `docs/decisions/` (copy `_template.md`) |
| Change build/test/deploy/run steps | the relevant `docs/guides/` page |
| Add/rename/remove config keys, env vars, or public API | `docs/reference/` |
| Add or move a source file that a doc's `related_code` points at | that doc's `related_code` front-matter |

Then regenerate derived indexes and adapter files:

```bash
maat sync
```

And validate before committing:

```bash
maat check     # fails on stale/broken/missing docs
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

## Human approval

Agents may draft documentation changes, but a human reviews them in the pull
request alongside the code. The CI check (`maat check`) enforces that docs
were updated; the reviewer confirms they are *correct*.

## Setup, build, and test commands

<!-- Fill these in so agents can run the project's checks. Examples: -->
<!--
- Install: `...`
- Test: `...`
- Lint: `...`
-->
