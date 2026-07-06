---
title: Architecture Decision Records
status: current
summary: Index of ADRs — the append-only log of why the system is the way it is.
---

# Decisions (ADRs)

An **Architecture Decision Record** captures one significant, hard-to-reverse
choice: the context, the decision, and its consequences. ADRs are
**append-only** — you do not edit an accepted decision to change its meaning.
To change direction, add a new ADR that supersedes the old one and set the old
one's status to `deprecated`.

Copy [`_template.md`](_template.md) to `NNNN-short-title.md` (next number).

## Log

Newest first:

- [0008 Brownfield adoption and BYO-agent retrospection](0008-brownfield-adoption-byo-agent.md)
- [0007 Agent skills as managed artifacts](0007-agent-skills-as-managed-artifacts.md)
- [0006 Distribution and version pinning](0006-distribution-and-versioning.md)
- [0005 Rewrite the CLI in Go, distribute a single static binary](0005-go-rewrite.md)
- [0004 Detect staleness via related_code timestamps](0004-related-code-staleness.md)
- [0003 AGENTS.md as the source of truth, with generated adapters](0003-agents-md-source-of-truth.md)
- [0002 Zero runtime dependencies](0002-zero-dependencies.md) *(deprecated — superseded by 0005)*
- [0001 Record architecture decisions](0001-record-architecture-decisions.md)
