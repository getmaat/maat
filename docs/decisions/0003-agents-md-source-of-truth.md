---
title: 0003 AGENTS.md as source of truth with generated adapters
status: current
summary: One canonical instruction file (AGENTS.md); every other agent's file is generated from it and verified in CI.
---

# 0003. AGENTS.md as the source of truth, with generated adapters

- **Status:** accepted
- **Date:** 2026-07-02

## Context

Every AI coding harness looks for its instructions in a different place:
Claude Code reads `CLAUDE.md`, GitHub Copilot reads
`.github/copilot-instructions.md`, Cursor reads `.cursor/rules/*.mdc`,
Windsurf reads `.windsurf/rules/*.md`, Gemini reads `GEMINI.md`, Hermes reads
`.hermes.md`, and a large and growing set read `AGENTS.md`. If a team maintains
these by hand they drift apart, and agents end up with contradictory guidance.

We want a single place to write instructions and a guarantee that every
harness sees the same thing.

## Decision

The root **`AGENTS.md`** is the single, canonical instruction file — chosen
because it is an open standard with the widest native adoption. Every other
agent-specific file is a **generated adapter** that points back to `AGENTS.md`
and `docs/llms.txt`. Adapters are produced by `maat sync` and their
correctness is enforced by `maat check` (drift detection), so they can never
silently diverge.

Adapters carry no original content: they are thin pointers (or, for Cursor's
`.mdc`, a minimal always-apply rule) wrapping a managed, regenerable region.

Which adapters are emitted is configured per-repo in `.maat.yml`.

## Consequences

- A team writes instructions once; every agent gets a consistent version.
- Adding support for a new harness means adding one adapter generator, not
  changing how anyone writes docs.
- Generated adapter files are checked into the repo (so agents that read them
  directly from a fresh clone work) but must not be hand-edited — CI will flag
  drift and tell the author to run `maat sync`.

## Alternatives considered

- **Symlinks** from each agent file to `AGENTS.md` — breaks on Windows, and the
  files have genuinely different required formats (Cursor needs YAML
  front-matter), so a plain link cannot serve all of them.
- **Tell users to configure each agent to read `AGENTS.md`** — not possible for
  harnesses with hard-coded filenames, and relies on per-developer setup rather
  than something committed to the repo.
