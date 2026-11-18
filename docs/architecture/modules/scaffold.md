---
title: Init & scaffold module
status: current
summary: What maat init stamps into a repository, where the embedded templates live, and how version-dependent values get into them.
related_code:
  - internal/maat/scaffold.go
  - internal/maat/templates
---

# Init & scaffold

## Responsibility

Stamp Ma'at into a repository: write the starter files (`AGENTS.md`, the
`docs/` tree, `.maat.yml`, the CI workflow, the module template), then run
`sync` once so every derived file exists. It also owns the **embedded
templates** that other modules render from — the adapter pointers and the
managed skills.

It is **not** responsible for keeping anything up to date after `init`. Once a
file is stamped it belongs to the user (scaffold files) or to `sync` (derived
files, see [Model & generators](model-generate.md)).

## Key files

- `internal/maat/scaffold.go`
  - `templatesFS` / `tmpl(name)` — every file under `internal/maat/templates/`
    is compiled into the binary via `//go:embed`; `tmpl` panics on a missing
    name, so a typo fails the first test run rather than a user's `init`.
  - `scaffoldFiles` — the ordered list of `{destination, template}` pairs
    `init` writes.
  - `skillDefs` — the agent skills Ma'at ships
    ([ADR 0007](../../decisions/0007-agent-skills-as-managed-artifacts.md)).
    They are rendered by `sync`, not by `init`, but registered here next to
    their templates.
  - `RunInit()` — fills placeholders, writes files that do not exist yet (all
    of them with `--force`), then runs `RunSync`.
  - `repoFromRemote()` / `repoFromURL()` — pre-fill `repo:` once from the git
    `origin` remote ([ADR 0013](../../decisions/0013-contract-edges-and-manifest.md)).
- `internal/maat/templates/` — the template files themselves. The scaffold
  templates are deliberately thinner than this repository's own docs, which
  are hand-written so the project dogfoods the framework.

## Interfaces / contracts

- **Placeholders** use `{{NAME}}` and are filled by `fill()`, longest key
  first (so `{{SUMMARY_INLINE}}` is not clobbered by `{{SUMMARY}}`):
  `PROJECT`, `SUMMARY`, `SUMMARY_INLINE`, `DATE`, `NAME`, `PATH`,
  `MAAT_VERSION`, `MAAT_ACTION_REF`, `ADAPTERS_BLOCK`, `REPO_LINE`. Skill
  templates use a different, single-brace syntax (`{maat_version}`,
  `{docs_dir}`) rendered by `skillContent()` in `generate.go`.
- **Existing files are never overwritten** without `--force`. Re-running
  `init` only fills gaps; skipped files are reported, and brownfield guidance
  is printed ([ADR 0008](../../decisions/0008-brownfield-adoption-byo-agent.md)).
- **Derived values are computed only when the file is actually written.** For
  example, `repo` is read from the remote only when `.maat.yml` is new (or
  `--force` is given), so `init` never claims to have set something it skipped.
- `init` exit code is 0 on success, including when files were skipped.

## Gotchas

- **Version-dependent values differ between release and source builds.**
  `scaffoldVersionPin()` and `scaffoldActionRef()` stamp the binary's own
  version into the generated CI workflow; a source build (`go run .`) cannot
  know a release, so it writes an empty pin and a `vX.Y.Z` placeholder. The
  same split drives the `maat-version:` stamp in generated skills (`dev` from
  source builds) — which is why this repository must be synced with
  `go run . sync`, never an installed release (see the
  [development guide](../../guides/development.md)).
- A placeholder that no `subs` entry fills stays in the output verbatim
  (`{{LIKE_THIS}}`). Add the key in `RunInit` when adding one to a template.
- The YAML templates (`config.yml`, `workflow.yml`) are plain text to Go; a
  placeholder that expands to YAML (like `ADAPTERS_BLOCK`) must produce valid
  YAML at the right indentation itself.
