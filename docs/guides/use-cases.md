---
title: When to use Ma'at
status: current
summary: The problems Ma'at solves, phrased as the questions people ask, with the feature that answers each one.
---

# When to use Ma'at

This page is for deciding whether Ma'at fits your repository. Each section
starts with a question people ask when they hit the problem, then says what
Ma'at does about it and where to read more. To set it up, see
[Adopting Ma'at in a repository](deployment.md).

## How do I keep AGENTS.md, CLAUDE.md and other agent instruction files in sync?

Every coding agent reads its own file: `CLAUDE.md`, `GEMINI.md`,
`.github/copilot-instructions.md`, `.cursor/rules/`, `.windsurf/rules/`,
`.hermes.md`. Copies drift as soon as someone edits one of them.

With Ma'at, `AGENTS.md` is the only file you edit. `maat sync` generates the
others from it, and `maat check` fails CI when a generated file no longer
matches its source. Tools that read `AGENTS.md` directly (Codex, opencode and
others) use it as is.

Read more: [ADR 0003](../decisions/0003-agents-md-source-of-truth.md),
[configuration reference](../reference/configuration.md).

## How do I find out when my docs are out of date with the code?

Each doc lists the code it describes in its front matter (`related_code`).
When a later commit changes that code but not the doc, `maat check` reports
the doc as stale. Staleness is judged from git history, so it works in CI and
is not fooled by checkouts, formatters or rebases that touch file timestamps.
Uncommitted edits count too, so you see the warning before you push.

Read more: [front-matter reference](../reference/frontmatter.md),
[ADR 0012](../decisions/0012-git-aware-staleness.md).

## How do I make CI fail when documentation drifts?

Run `maat check` as a required job on every pull request, with the published
GitHub Action, the reusable workflow, or the installed binary on any other CI.
It validates front matter, links, `related_code` paths, staleness, contracts
and generated files, and it is offline and deterministic: the same commit
always gets the same verdict.

Read more: [Adopting Ma'at, step 4](deployment.md#4-wire-up-ci),
[check engine](../architecture/modules/check.md),
[troubleshooting](troubleshooting.md).

## Why do AI coding agents keep acting on outdated instructions?

An agent trusts what the repository tells it. If a doc says "tokens expire
after 15 minutes" and the code changed to 5, the next agent session builds on
the wrong number. Ma'at gives agents a machine-readable index of every doc
(`docs/llms.txt`), puts the documentation update protocol into the
instruction file itself, and makes CI catch the drift that slips through.

Read more: [maintaining the documentation](../meta/maintenance.md),
[ADR 0009](../decisions/0009-contract-as-managed-block.md).

## How do I check API contracts between repositories without a central service?

A doc can declare the interfaces it provides (an HTTP API, an event, a
schema, with a version and an optional spec file) and the ones it consumes
from other repositories. Every repository publishes its edges as
`.maat/manifest.json`. A consumer runs `maat contracts update` to lock the
versions it uses into `.maat/contracts.lock.json`, and `maat check`
validates against that lockfile offline. A scheduled job with
`maat contracts update --check` tells you when a provider has moved on, so a
breaking change shows up as a reviewable lockfile diff instead of a surprise
red build.

No server, catalog or shared database is involved: repositories read each
other's committed manifests.

Read more: [ADR 0013](../decisions/0013-contract-edges-and-manifest.md),
[ADR 0014](../decisions/0014-cross-repo-contracts-via-lockfile.md),
[manifest reference](../reference/manifest.md),
[lockfile reference](../reference/contracts-lock.md).

## Can I add this to an existing repository with docs already in it?

Yes. `maat init` never overwrites files you already have. If your repository
has an `AGENTS.md`, your prose stays and Ma'at adds its rules in a clearly
marked managed block. For deriving architecture docs and decision records
from an established codebase, `init` ships a `retrospect` skill your own AI
agent can follow.

Read more: [ADR 0008](../decisions/0008-brownfield-adoption-byo-agent.md),
[CLI reference](../reference/cli.md).

## When is Ma'at not the right tool?

- **You want a documentation website.** Ma'at keeps repository-local docs
  correct; it does not render sites. Use it alongside a site generator.
- **Your docs live outside the repository** (a wiki, Confluence, Notion).
  Ma'at checks Markdown that is versioned with the code.
- **You need a service catalog with a UI** for humans to browse. Ma'at checks
  contracts in each repository's CI; it has no server or dashboard.
