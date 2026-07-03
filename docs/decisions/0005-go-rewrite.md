---
title: 0005 Rewrite the CLI in Go, distribute a single static binary
status: current
summary: The CLI is rewritten in Go and shipped as one static binary; the Python reference is retired. Supersedes 0002.
---

# 0005. Rewrite the CLI in Go, distribute a single static binary

- **Status:** accepted
- **Date:** 2026-07-02
- **Supersedes:** [0002 Zero runtime dependencies](0002-zero-dependencies.md)

## Context

Ma'at's headline promise is that *any* repository — in any language, on any
CI image — can adopt it with zero friction. [ADR 0002](0002-zero-dependencies.md)
pursued that with a stdlib-only Python implementation and a hand-written YAML
subset parser, explicitly to avoid a `pip install` step.

In practice the Python implementation did **not** deliver frictionless
adoption, because the interpreter is itself a dependency:

- Running the tool still requires Python 3.8+ on the machine. A team on a pure
  Go/Rust/Node stack may have no interpreter at all, and "vendor this Python
  package into your repo" is friction precisely where we promised none.
- Avoiding PyYAML forced us to own a bespoke YAML *subset* parser
  (`codedoc/_yaml.py`) — a correctness and maintenance liability we took on
  *only* to dodge a runtime dependency.
- The interpreter-as-dependency leaked in concrete ways: an editable install
  needed a modern `pip` (PEP 660), and a `PYTHONPATH` workaround tripped a
  security scan.

Ma'at is gate/infrastructure tooling (like `gh`, `shellcheck`,
`golangci-lint`, `ripgrep`), not a doc renderer that runs inside a project's
own environment (like mkdocs or sphinx). That category overwhelmingly ships as
a single compiled binary, for exactly the omnipresence reason above.

## Decision

Rewrite the CLI in **Go** and distribute it as a **single static binary**.

- One statically linked executable — no interpreter, no virtualenv, no
  `PYTHONPATH`, no `pip`. `maat` just runs. This makes "zero dependencies"
  *literally* true rather than "zero pip dependencies, but you need Python."
- Cross-compile to every OS/arch from one machine, for GitHub Releases,
  `go install`, and Homebrew. The binary drops into any CI image regardless of
  the project's language.
- Scaffold templates are compiled into the binary with `//go:embed`, so `init`
  needs no companion files on disk.
- The **convention** Ma'at defines — `AGENTS.md` as source of truth, the
  `docs/` structure, the front-matter schema, the update protocol, and the
  generated adapter formats — is unchanged and language-independent. Only the
  *tool* that automates it was rewritten.

The Go implementation reproduces the Python CLI's behaviour exactly: the same
`init` / `sync` / `check` subcommands, identical generated output (verified
byte-for-byte against the retired Python implementation), the same `check`
findings and exit codes, and the underscore-prefixed template exclusion. The
19-case Python test suite was ported to Go as the conformance spec.

The `pip`-avoidance rationale of ADR 0002 is now obsolete: a Go dependency such
as a YAML library compiles *into* the binary as a build-time dependency with
zero runtime cost. The bespoke YAML subset survives the port only because it is
already written and small, not because a runtime dependency must be avoided.

## Consequences

- Users get a single binary with no runtime prerequisites; "any repo, any CI,
  any language" is now literally true.
- Building or contributing to Ma'at requires the **Go toolchain** (1.24+)
  instead of a Python interpreter. This raises the bar to *build* the tool
  while lowering the bar to *run* it — the right trade for gate tooling.
- The Python implementation (`codedoc/`, `pyproject.toml`, `tests/`) is retired.
  It remains in git history as the validated reference the Go port was checked
  against.
- ADR 0002's "zero runtime dependencies" goal is **preserved and strengthened**
  (a static binary has none), but its "stdlib-only Python 3.8+" mechanism is
  superseded by this ADR.

## Alternatives considered

- **Keep the Python implementation** — rejected: the interpreter is a
  dependency that undermines the core promise, and we were maintaining a YAML
  parser solely to work around Python packaging.
- **Rewrite in Rust** — a viable single-binary option with the same
  distribution win. Rejected for this tool because Ma'at mostly walks files
  and splices text; Go is less ceremony, faster to write, and has the stronger
  first-party tooling ecosystem for a CLI of this shape. Revisit only if
  performance or a Rust-native ecosystem need emerges.
- **Ship Python as a frozen binary** (PyInstaller/Nuitka) — produces large,
  platform-fragile artifacts and still bundles an interpreter; a compiled
  language is the cleaner answer.
