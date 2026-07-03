---
title: Development guide
status: current
summary: How to set up, run, and contribute to Ma'at locally.
---

# Development guide

## Prerequisites

- **Go 1.24+** — to build and test Ma'at. There are no other build- or
  run-time dependencies (see [ADR 0005](../decisions/0005-go-rewrite.md)).
- Nothing else. The YAML subset parser is hand-written and vendored in the
  package, so there are no third-party modules to fetch.

## Setup

```bash
git clone <this-repo>
cd maat
go run . --help      # runs straight from the clone
```

To produce a standalone binary you can drop on your `PATH`:

```bash
go build -o maat .
./maat --help
```

## Everyday workflow

The CLI entry point is `main.go` at the repository root; the engine is the
`internal/maat/` package. The layout and responsibilities are described in the
[architecture overview](../architecture/overview.md); read it before making
structural changes.

A normal change loop:

```bash
# 1. make your code change under internal/maat/ (or main.go)
# 2. update the matching doc(s) — see AGENTS.md "update protocol"
go run . sync      # regenerate derived files if docs changed
go run . check     # validate docs (the CI gate)
go test ./...                  # run the tests
```

Both `check` and the tests must be green before you open a pull request.

## Coding conventions

- Standard library only — no third-party runtime dependencies.
- Keep generators pure (no I/O); all disk writes live in `sync.go`.
- Match the existing module boundaries; see
  [conventions](../meta/conventions.md).
