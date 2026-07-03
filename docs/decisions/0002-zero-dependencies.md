---
title: 0002 Zero runtime dependencies
status: deprecated
summary: Superseded by ADR 0005. The zero-runtime-dependency goal is preserved by the Go static binary; the stdlib-only-Python mechanism is retired.
---

# 0002. Zero runtime dependencies

- **Status:** deprecated — superseded by [0005 Rewrite the CLI in Go](0005-go-rewrite.md)
- **Date:** 2026-07-02

> **Superseded (2026-07-02).** This ADR's *goal* — zero runtime dependencies —
> is preserved and strengthened by [ADR 0005](0005-go-rewrite.md): a single Go
> static binary has no runtime dependencies at all. Its *mechanism* —
> stdlib-only Python 3.8+ with a hand-written YAML subset — is retired, because
> the Python interpreter is itself a runtime dependency. The original text is
> kept below unaltered, as ADRs are append-only.

## Context

Ma'at's value is that *any* repository and *any* agent harness can adopt it.
A tool that requires `pip install` — and therefore a resolved, compatible
dependency environment — is friction at exactly the moment we want none:
the first time someone drops Ma'at into an existing project, and every time
a CI job or an agent shells out to it.

The most tempting dependency is a YAML library (PyYAML) for reading
front-matter and `.maat.yml`.

## Decision

Ma'at will have **zero runtime dependencies** and target **Python 3.8+**
using only the standard library. Rather than depend on PyYAML we ship a small
YAML *subset* parser/emitter (`codedoc/_yaml.py`) covering exactly the
front-matter and config syntax Ma'at uses.

Test-time and lint-time tools (pytest, type checkers) are allowed as
development dependencies, because they are never needed to *run* the tool.

## Consequences

- Ma'at can be vendored as a directory and run with `python3 -m codedoc`
  anywhere, including minimal CI images, with no install step.
- We own a small amount of YAML-parsing code and must keep it within the subset
  it supports; exotic YAML in front-matter is intentionally unsupported.
- Contributors must resist adding convenience dependencies. A proposal to add
  one should supersede this ADR with explicit justification.

## Alternatives considered

- **Depend on PyYAML** — the obvious choice, rejected because it reintroduces
  the install-and-resolve friction the whole project exists to avoid.
- **Require the docs to use JSON front-matter** (stdlib `json`) — worse
  ergonomics for humans writing docs; YAML front-matter is the ecosystem norm.
