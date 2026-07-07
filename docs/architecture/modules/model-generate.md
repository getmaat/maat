---
title: Model & generators module
status: current
summary: Scanning the docs tree into a model and rendering derived artifacts from it.
related_code:
  - internal/maat/model.go
  - internal/maat/generate.go
  - internal/maat/sync.go
---

# Model & generators

## Responsibility

Turn the `docs/` tree into an in-memory model, and render every *derived*
artifact from that model: `llms.txt`, the per-agent adapter files, the
managed navigation block inside `docs/index.md`, the managed agent skills
under `.maat/skills/` (plus their vendor copies — see
[ADR 0007](../../decisions/0007-agent-skills-as-managed-artifacts.md)), and the
managed **maintenance-contract block** spliced into the instruction file — the
documentation update protocol, the front-matter schema, and the skills
discovery list (see
[ADR 0009](../../decisions/0009-contract-as-managed-block.md)).

## Key files

- `internal/maat/model.go` — `ScanModel(root, docsDir)` walks the
  docs directory, parses each Markdown file's front-matter, and produces a
  `DocsModel` holding `Document` values (with `Rel`, `Meta`, `Status`,
  `RelatedCode`, and helpers to extract Markdown links). Files whose name begins
  with `_` (e.g. `decisions/_template.md`) are treated as templates/partials and
  skipped, so they never appear in indexes or validation while remaining valid
  link targets on disk.
- `internal/maat/generate.go` — pure rendering functions: `llmsTxt()`,
  `indexNav()`, `adapterContent()`, `skillContent()` (managed agent skills,
  ADR 0007), `contractBlock()` (the maintenance-contract block spliced into the
  instruction file — update protocol, front-matter schema, skills list, ADR
  0009), plus `splice()` which inserts generated text between
  `<!-- maat:begin -->` / `<!-- maat:end -->` markers without disturbing
  hand-written content around them.
- `internal/maat/sync.go` — `expectedArtifacts()` composes the generators
  into the full ordered `{path: content}` map; `writeArtifacts()` writes only
  changed files.

## Interfaces / contracts

- `expectedArtifacts(model, cfg, root)` is the **single definition of
  "correct"** for every generated file. Both `sync` (writer) and `check`
  (drift detector) consume it, guaranteeing they agree.
- Generators are pure functions of the model + config; they perform no I/O.
  All disk access lives in `sync.go`, which keeps generation testable.
- `splice()` preserves everything outside the managed markers, so a human may
  freely add prose above/below the generated block in `index.md`, the adapters,
  and the instruction file. This is what lets the maintenance-contract block
  (ADR 0009) drop into a brownfield `AGENTS.md` that `init` preserved whole:
  the block is appended (or replaced in place if its markers already exist)
  while the team's hand-written sections are left untouched.

## Gotchas

- `.mdc` (Cursor) adapters and the skill files are generated **whole**, not
  spliced — `.mdc` because its YAML front-matter cannot host HTML comment
  markers, skills because they are whole-file owned by Ma'at (ADR 0007).
  Editing either by hand will be reported as drift. Team-authored skills under
  `.maat/skills/` with other names are never touched.
- Adapter relative paths depend on the file's directory depth (e.g.
  `.github/copilot-instructions.md` points at `../AGENTS.md`). That math lives
  in `adapterCtx()` in `sync.go`.
