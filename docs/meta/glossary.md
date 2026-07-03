---
title: Glossary
status: current
summary: Definitions of Ma'at's domain terms.
---

# Glossary

**Adapter file** — a generated, agent-specific instruction file (e.g.
`CLAUDE.md`, `.github/copilot-instructions.md`) that points back to the
canonical `AGENTS.md`. Produced by `sync`, verified by `check`.

**ADR (Architecture Decision Record)** — a numbered, append-only document
capturing one significant technical decision: its context, the decision, and
its consequences. Lives in `docs/decisions/`.

**Canonical instruction file** — the single source of truth for agent
instructions, `AGENTS.md` at the repo root. All adapters derive from it.

**Derived / generated file** — any file `sync` produces from the source of
truth (`llms.txt`, adapters, the index navigation block). Never hand-edited;
`check` reports drift if one differs from what `sync` would write.

**Docs model** — the in-memory representation of the `docs/` tree built by
`DocsModel.scan()`: one `Document` per Markdown file with its parsed
front-matter.

**Drift** — a generated file on disk disagreeing with freshly generated
content, i.e. someone edited it by hand or forgot to run `sync`.

**Front-matter** — the YAML block delimited by `---` at the top of every doc,
carrying `title`, `status`, `summary`, and optional `related_code`.

**llms.txt** — the machine-readable index of the docs tree, following the
[llms.txt](https://llmstxt.org) convention; the first file an agent reads.

**Managed region** — the span between `<!-- maat:begin -->` and
`<!-- maat:end -->` markers in an otherwise hand-written file, into which
`sync` splices generated content.

**`related_code`** — a front-matter list naming the source files a doc
describes; powers staleness and orphaned-code detection.

**Staleness** — a doc whose `related_code` source file has a newer modification
time, suggesting the doc may not reflect the current code.
