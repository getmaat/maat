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

<!--
The Documentation update protocol, the front-matter schema, and the skills
list are Ma'at framework invariants, so they are maintained for you in a
generated managed block that `maat sync` splices in below (edit the source
docs, not that block). This keeps the contract identical in every Ma'at repo
and lets it drop non-destructively into a brownfield AGENTS.md that already
existed. Keep this file's hand-written sections (above) about *this* project.
-->

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
