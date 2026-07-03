---
title: Maintaining the documentation
status: current
summary: How the docs stay in sync with the code — the lifecycle and tooling.
---

# Maintaining the documentation

This documentation is kept current by three actors and one tool.

## The actors

1. **AI agents** update the relevant docs in the same change they make to the
   code, following the "Documentation update protocol" in the root
   `AGENTS.md`. A human approves the result in review.
2. **Developers** editing by hand do the same, using the templates in
   `templates/` and the conventions in `conventions.md`.
3. **CI/CD** runs `maat check` on every pull request and fails the build if
   documentation is stale, links are broken, or generated files drifted.

## The tool

`maat` is a zero-dependency Go CLI, distributed as a single static binary:

- `maat sync` — regenerate `llms.txt`, `docs/index.md` navigation, and the
  per-agent adapter files from the docs tree.
- `maat check` — validate front-matter, links, `related_code` paths,
  staleness, and drift. This is the CI gate.
- `maat init` — scaffold this structure into a repository.

## Staleness detection

When a doc declares `related_code`, `maat check` compares modification
times. If the code is newer than the doc, it flags the doc for review. After
refreshing the doc, re-save it (or run `maat sync`) so its timestamp
advances.
