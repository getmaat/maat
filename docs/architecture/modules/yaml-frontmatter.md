---
title: YAML & front-matter module
status: current
summary: The parsing layer — a hand-rolled YAML subset and Markdown front-matter reading.
related_code:
  - internal/maat/yaml.go
  - internal/maat/frontmatter.go
---

# YAML & front-matter

## Responsibility

Provide just enough YAML to read document front-matter and `.maat.yml` with a
small, hand-rolled parser instead of a third-party YAML library. This module is the foundation the rest of Ma'at
builds on; it is intentionally the only place that touches YAML syntax.

It is **not** a general-purpose YAML implementation. It supports the subset
that Ma'at's front-matter and `.maat.yml` actually use, and nothing more.

## Key files

- `internal/maat/yaml.go` — a small recursive parser for a YAML subset:
  scalars (with `int`/`float`/`bool`/`null` coercion), quoted strings, flow
  lists (`[a, b]`), block lists (`- item`, indented under their key or at the
  same indentation as it), nested mappings by indentation, and lists of
  mappings (`- id: x` followed by keys aligned with `id`, or a bare `-` with
  the mapping on the following lines). Exposes `yamlParse(text) (any, error)`.
  The tool only ever *reads* YAML, so there is no emitter.
- `internal/maat/frontmatter.go` — splits a Markdown file into its
  `---`-delimited front-matter block and body. Exposes
  `fmSplit(text) (meta map[string]any, body string, err error)` and the
  file-reading convenience `fmRead(path)`. A leading byte-order mark is
  tolerated, and the single blank line after the closing fence is dropped
  from the body.

## Interfaces / contracts

- `fmSplit("")` returns an empty map and no error — an empty or absent block is
  valid, never an error.
- Malformed front-matter **is** an error (`FrontMatterError`): a `---` fence
  that opens but never closes, or a block that parses to something other than
  a mapping. Failing loudly lets authors notice typos early.
- **The parser never drops input silently.** Every line must belong to the
  parsed structure; a line it cannot place (mis-indented, or an unsupported
  construct such as a `|`/`>` block scalar) is a `YAMLError` naming the line
  number. Constructs it does not interpret but can hold as text — a value
  starting with `{`, `&` or `*`, for example — are kept as plain strings, so
  template placeholders such as `{{PROJECT}}` keep working.
- Parsing is read-only and deterministic: Ma'at never re-emits parsed YAML.
  Generated Markdown is produced by templating and spliced between managed
  markers, so front-matter the tool did not author is preserved verbatim and
  files do not thrash in version control.
- Front-matter keys used elsewhere: `title`, `status`, `summary`,
  `related_code`. See [front-matter reference](../../reference/frontmatter.md).

## Gotchas

- The parser assumes spaces, not tabs, for indentation (as does YAML proper).
- Only the subset above is supported. If a doc needs a structure the parser
  does not handle, extend `yaml.go` deliberately and add a test. Build-time Go
  dependencies are permitted since
  [ADR 0011](../../decisions/0011-build-time-go-dependencies.md), but that ADR
  keeps the hand-rolled subset on purpose: front-matter is a small, stable
  format Ma'at can scope down safely, so a general-purpose YAML library would
  add surface without benefit. Revisit only if the needed subset grows
  substantially.
