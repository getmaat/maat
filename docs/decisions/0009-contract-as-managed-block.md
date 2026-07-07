---
title: 0009 Maintenance contract as a managed block in the instruction file
status: current
summary: The documentation update protocol, front-matter schema, and skills index are framework invariants, so they are rendered into the managed maat:begin/end block spliced into the instruction file — not hand-written scaffold prose — so a brownfield AGENTS.md that init preserved still gains the contract non-destructively.
---

# 0009. Maintenance contract as a managed block in the instruction file

- **Status:** accepted
- **Date:** 2026-07-07
- **Relates to:** [0003 AGENTS.md as the source of truth, with generated adapters](0003-agents-md-source-of-truth.md), [0007 Agent skills as managed artifacts](0007-agent-skills-as-managed-artifacts.md), [0008 Brownfield adoption and BYO-agent retrospection](0008-brownfield-adoption-byo-agent.md)

## Context

ADR 0008 made `init` non-destructive: an existing `AGENTS.md` is a *scaffold
file*, so it is skipped whole and never overwritten. That protects the team's
hand-written instruction file — but it also means a brownfield repo never
receives the **documentation update protocol**: the "a change is not complete
until its docs are updated; then run `maat sync` / `maat check`" contract that
is the entire point of adopting Ma'at. Without it, `maat check` guards a habit
the instruction file never taught. Ma'at was effectively only complete on a
greenfield repo; on the far more common brownfield repo the UX was degraded.

There was already an inconsistency to build from. The **skills discovery list**
(ADR 0007) is *not* a scaffold file — it is rendered into a managed
`<!-- maat:begin --> … <!-- maat:end -->` region and spliced into the
instruction file by `sync`, which preserves everything outside the markers. So
Ma'at *already* injected a managed block into a pre-existing `AGENTS.md`. It
just injected the least essential part (a pointer to skills) while omitting the
most essential part (the update protocol).

The protocol, the front-matter schema, and the skills list share a property the
project overview does not: they are **framework invariants**. They reference
Ma'at's own commands and conventions and read identically in every Ma'at repo.
That is exactly the kind of content the managed-block mechanism exists for.

## Decision

**Render Ma'at's maintenance contract as generated managed content, not as
hand-written scaffold prose.**

- The documentation update protocol, the front-matter schema, and the skills
  discovery list are rendered by a single generator (`contractBlock`) into the
  managed `maat:begin/end` region spliced into the instruction file. Paths are
  parameterized on `docs_dir` so a non-default docs directory gets correct links.
- This block is emitted **unconditionally** — even for a repo with zero skills —
  so the contract always lands. It replaces the skills-only block that was
  previously gated behind "at least one skill exists".
- The scaffold template (`templates/AGENTS.md`) no longer hand-writes the
  protocol or the front-matter section. Those sections now live only in the
  generated block; the template body keeps genuinely project-specific prose
  (overview, docs map, human-approval note, build commands).
- Consequently a brownfield `AGENTS.md` — skipped as a scaffold file — is still
  spliced with the contract by the `sync` that `init` runs, non-destructively.
  It therefore appears as both `skip` (scaffold preserved) and `gen` (block
  inserted); `init`'s guidance explains this.

This **refines ADR 0008 rather than superseding it.** 0008's split still holds —
CLI = deterministic, skill = procedure, agent = intelligence. This ADR observes
that a slice of the "content" 0008 delegated wholesale to the agent is not
intelligence work at all but a fixed invariant, and moves that slice into the
deterministic CLI. What is left to the retrospect skill is exactly the part that
needs judgment: the project overview, the architecture docs, and the ADRs.

## Consequences

- **Brownfield adoption is materially better.** Any repo, however old, gains the
  update protocol the moment it runs `init` (or any later `sync`), without a
  human or agent copying prose by hand.
- **The contract self-heals and stays uniform.** Because it is generated, an
  improved protocol table ships with a binary upgrade and propagates on the next
  `sync`; `check` flags drift. No repo drifts to a stale, hand-forked protocol.
- **Greenfield `AGENTS.md` structure shifts.** The contract now renders at the
  end of the file (where the managed block is appended) instead of mid-body.
  This is the honest boundary: hand-written project prose above, generated
  framework contract below.
- **Teams can no longer reword the protocol table in place.** Accepted: it is a
  framework invariant, and hand-edits inside the managed markers are overwritten
  by `sync` (and flagged by `check`) — the same contract the skills block has
  always carried. Project-specific doc-routing rules belong in a
  `docs/guides/` page or the hand-written body, not the invariant table.
- **The adapter pointers stay valid.** They reference the instruction file
  "under \"Documentation update protocol\""; the generator keeps that exact
  heading, so the cross-reference still resolves.
- Ma'at itself dogfoods this: its own `AGENTS.md` drops the hand-written
  protocol/front-matter sections and takes them from the generated block.

## Alternatives considered

- **Print the protocol to stdout on a skipped `AGENTS.md`.** Rejected: transient
  — it scrolls away and persists nothing into the repo, so the contract is still
  absent from the file agents actually read.
- **Write a reference copy to a side file (e.g. `AGENTS.maat.md`) and point at
  it.** Rejected: leaves the real instruction file incomplete and adds a manual
  copy step and an extra artifact, for content that can be injected safely.
- **Leave it to the retrospect skill (status quo of ADR 0008).** Rejected as the
  default: it makes a fixed invariant contingent on an interactive agent session
  the team may never run. The skill still owns the judgment-heavy merge; it no
  longer has to hand-transcribe an invariant.
- **A second, separately-marked managed region for the protocol.** Deferred:
  one managed region per file is simpler and the splice machinery supports only
  one marker pair today; the protocol and skills read fine as one contiguous
  "maintenance contract" section.
