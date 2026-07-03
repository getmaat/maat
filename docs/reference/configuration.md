---
title: Configuration reference
status: current
summary: Every .maat.yml key, its default, and its effect.
related_code:
  - internal/maat/config.go
---

# Configuration reference

Ma'at reads an optional `.maat.yml` at the repository root. Every key has
a default, so the file may be partial or absent entirely.

```yaml
project_name: Ma'at
project_summary: One-line description used in generated indexes.

docs_dir: docs
instructions_file: AGENTS.md

adapters:
  - claude
  - hermes
  - copilot
  - cursor
  - windsurf
  - gemini

required_frontmatter:
  - title
  - status

statuses:
  - current
  - draft
  - deprecated

check:
  orphaned_code_is_error: true
  broken_links_is_error: true
  drift_is_error: true
  staleness: warn          # off | warn | error
  ignore_code_paths: []
```

## Top-level keys

| Key | Default | Effect |
|-----|---------|--------|
| `project_name` | directory name | Name used in `llms.txt` and generated headings |
| `project_summary` | empty | One-line summary in `llms.txt` / index |
| `docs_dir` | `docs` | Directory the docs tree lives in |
| `instructions_file` | `AGENTS.md` | The canonical instruction file adapters point at |
| `adapters` | see below | Which agent adapter files `sync` generates |
| `required_frontmatter` | `[title, status]` | Keys every doc must define |
| `statuses` | `[current, draft, deprecated]` | Allowed `status` values |

## `adapters`

Each entry generates one agent-specific file, kept in sync with
`instructions_file`:

| Value | Generates |
|-------|-----------|
| `claude` | `CLAUDE.md` |
| `hermes` | `.hermes.md` |
| `copilot` | `.github/copilot-instructions.md` |
| `cursor` | `.cursor/rules/maat.mdc` |
| `windsurf` | `.windsurf/rules/maat.md` |
| `gemini` | `GEMINI.md` |

Remove any your team does not use; `sync` will stop generating them (delete the
stale file once). See
[ADR 0003](../decisions/0003-agents-md-source-of-truth.md).

## `check`

| Key | Default | Effect |
|-----|---------|--------|
| `orphaned_code_is_error` | `true` | Missing `related_code` path fails the build |
| `broken_links_is_error` | `true` | Broken internal link fails the build |
| `drift_is_error` | `true` | A generated file out of sync fails the build |
| `staleness` | `warn` | `off` \| `warn` \| `error` for code-newer-than-doc |
| `ignore_code_paths` | `[]` | Path prefixes exempt from staleness/orphan checks |
