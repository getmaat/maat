---
title: 0008 Brownfield adoption and BYO-agent retrospection
status: current
summary: Adopting Ma'at in an existing repository is non-destructive by default; deriving documentation from an established codebase is delegated to the user's own AI agent via a shipped retrospect skill, with the CLI providing only deterministic facts.
---

# 0008. Brownfield adoption and BYO-agent retrospection

- **Status:** accepted
- **Date:** 2026-07-03
- **Relates to:** [0003 AGENTS.md as the source of truth, with generated adapters](0003-agents-md-source-of-truth.md), [0007 Agent skills as managed artifacts](0007-agent-skills-as-managed-artifacts.md)
- **Refined by:** [0009 Maintenance contract as a managed block](0009-contract-as-managed-block.md) — the update protocol, which this ADR left to the agent as "content", is now recognized as a framework invariant and injected into the instruction file as a managed block.

## Context

`maat init` in a fresh repository is the easy case: scaffold everything, the
team fills in content. Long-lived repositories are the hard — and far more
common — case, and they raise problems the greenfield flow never meets:

- The repo may already have an `AGENTS.md`, a `CLAUDE.md`, an ADR directory,
  or an established docs tree. Overwriting a hand-crafted `AGENTS.md` would
  destroy the very artifact ADR 0003 declares the source of truth.
- A ten-year-old repo fails every check on day one. A hard red gate on the
  first commit kills adoption; teams need a way to tighten incrementally.
- The valuable work — *deriving* an accurate `AGENTS.md`, architecture docs,
  and decision records from an established codebase — requires judgment and
  interviewing the developers. That is intelligence work, not tooling work.

The tempting shortcut is a `maat retrospect` command that calls an LLM to
write the docs. That would pull API-key management, provider churn, cost, and
non-determinism into a tool whose entire value is being deterministic,
offline, and dependency-free (ADR 0005/0006). It is also redundant: every team
adopting Ma'at already works with an AI agent — that is the premise of the
framework.

## Decision

Split brownfield adoption into three responsibilities:

**CLI = deterministic. Skills = procedure. The user's agent = intelligence.**

### 1. `init` is non-destructive and idempotent

`maat init` never overwrites an existing file unless `--force` is given (this
holds today; this ADR promotes it from behavior to contract). Re-running
`init` on any repository — fresh or ancient — only fills in what is missing
and reports what it skipped. There is no separate `adopt` command: brownfield
adoption *is* `init`, and the skip-and-report output *is* the gap inventory.

### 2. No LLM in the binary — a `retrospect` skill instead

Ma'at ships no LLM integration, no API keys, no agent harness. Instead, `init`
scaffolds a **`retrospect` skill** (per ADR 0007) that the team's own agent
executes. The skill encodes Ma'at's opinionated procedure:

1. Run `maat check` and read `init`'s skip report to inventory gaps
   deterministically.
2. Explore the codebase; **interview the developer** about intent,
   conventions, and constraints that code alone cannot reveal.
3. Derive `AGENTS.md` content, architecture docs, and decision records in
   Ma'at's format; run `maat sync` and re-run `maat check` until green.

The CLI supplies facts, the skill supplies the procedure, the agent supplies
the judgment. Because the skill is a managed artifact, the procedure evolves
with the pinned framework version instead of freezing at adoption time.

### 3. Retroactive ADRs are marked, and scoped to living decisions

ADRs written years after the fact are archaeology, not records — the context
is reconstructed, sometimes wrongly. The retrospect procedure therefore:

- marks derived records with `status: retrospective`, so a future reader knows
  the provenance differs from a decision captured at decision time;
- documents only decisions that **still constrain the codebase today** ("why
  we still use X"), not a full historical backfill of superseded choices.

### 4. Adoption is a ratchet, not a wall

The scaffolded configuration errs toward warnings so a brownfield repo gets a
green-ish first run and tightens knob by knob (`.maat.yml`'s `check` section
already supports per-rule severity). The retrospect skill's final step is
raising severities as gaps close. CI stays the authority (ADR 0006) — the
ratchet only ever tightens.

## Consequences

- **Adoption is safe by default.** No hand-written file is ever destroyed by
  `init`; the worst outcome of trying Ma'at is some new files to delete.
- **Ma'at stays deterministic and dependency-free.** No API keys, no provider
  coupling, no non-reproducible command output; ADR 0005's promises hold.
- **The derived docs are as good as the team's agent.** Accepted: quality
  intelligence is the agent's job, and the skill + deterministic gap report
  raise the floor by giving any agent the same procedure and the same facts.
- **The retrospect procedure is versioned.** Improving it is a template change
  shipped with the next release, propagated by the ADR 0007 ratchet — not a
  prompt lost in a wiki.
- `retrospect` runs are interactive (the skill interviews the developer), so
  they happen in a pair-programming session, not CI. That is deliberate:
  unattended doc generation is exactly the low-trust output ADR 0003's
  human-curated source of truth guards against.
- Future `init` output should stay machine-legible (stable `create`/`skip`
  line format), because the skill consumes it as data.

## Alternatives considered

- **A `maat retrospect` command with built-in LLM access.** Rejected: imports
  key management, provider churn, cost, and non-determinism into a
  deterministic tool; redundant given every adopting team already has an
  agent (bring-your-own-agent).
- **A separate `maat adopt` command.** Rejected: `init` is already idempotent
  and non-destructive, so a second command would duplicate it; one entry point
  is easier to document and discover. Revisit if brownfield-only behavior
  (e.g. a baseline file) ever diverges from what `init` can express.
- **A deterministic `retrospect` gap-report subcommand.** Deferred, not
  rejected: `init`'s skip report plus `maat check` already expose the needed
  facts; a dedicated report can be added if the skill proves to need richer
  input (e.g. git-history analysis).
- **Baseline/grandfather files** (eslint-style suppression lists). Deferred:
  per-rule severity in `.maat.yml` covers incremental adoption today with one
  fewer artifact; a baseline file can be added when a repo too large for
  rule-level granularity shows up.
- **Full historical ADR backfill.** Rejected: reconstructed context presented
  as contemporaneous record misleads future readers; only living constraints
  are worth deriving.
