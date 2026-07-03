---
title: YAML & front-matter module
status: current
summary: The dependency-free parsing layer — a YAML subset and Markdown front-matter I/O.
related_code:
  - internal/maat/yaml.go
  - internal/maat/frontmatter.go
---

# YAML & front-matter

## Responsibility

Provide just enough YAML to read and write document front-matter **without any
third-party dependency**. This module is the foundation the rest of Ma'at
builds on; it is intentionally the only place that touches YAML syntax.

It is **not** a general-purpose YAML implementation. It supports the subset
that Ma'at's front-matter and `.maat.yml` actually use, and nothing more.

## Key files

- `internal/maat/yaml.go` — a small recursive parser for a YAML subset:
  scalars (with `int`/`float`/`bool`/`null` coercion), quoted strings, flow
  lists (`[a, b]`), block lists (`- item`), and nested mappings by
  indentation. Exposes `yamlParse(text) (any, error)`. The tool only ever
  *reads* YAML, so there is no emitter.
- `internal/maat/frontmatter.go` — splits a Markdown file into its
  `---`-delimited front-matter block and body. Exposes
  `fmSplit(text) (meta map[string]any, body string, err error)` and the
  file-reading convenience `fmRead(path)`.

## Interfaces / contracts

- `fmSplit("")` returns an empty map and no error — an empty or absent block is
  valid, never an error.
- Parsing is read-only and deterministic: Ma'at never re-emits parsed YAML.
  Generated Markdown is produced by templating and spliced between managed
  markers, so front-matter the tool did not author is preserved verbatim and
  files do not thrash in version control.
- Front-matter keys used elsewhere: `title`, `status`, `summary`,
  `related_code`. See [front-matter reference](../../reference/frontmatter.md).

## Gotchas

- The parser assumes spaces, not tabs, for indentation (as does YAML proper).
- Only the subset above is supported. If a doc needs a structure the parser
  does not handle, extend `yaml.go` deliberately and add a test — do not pull in
  a general-purpose YAML library (see
  [ADR 0005](../../decisions/0005-go-rewrite.md), which supersedes
  [ADR 0002](../../decisions/0002-zero-dependencies.md)).
