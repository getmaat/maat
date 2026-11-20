---
title: Environment variables
status: current
summary: Environment variables Ma'at reads.
related_code:
  - internal/maat/fetch.go
---

# Environment variables

Ma'at's **behavior** — what it validates, what exit code it returns, what
content it writes — is determined entirely by command-line arguments and the
`.maat.yml` file at the repository root (see the
[configuration reference](configuration.md)), never by ambient environment
state. If a future feature needs configuration, it belongs in `.maat.yml`,
not the environment.

Two variables affect **presentation only** — never behavior:

| Variable | Effect |
|----------|--------|
| `NO_COLOR` | Any non-empty value disables ANSI styling, even in a real terminal. |
| `CLICOLOR_FORCE` | Any value other than `0` forces ANSI styling on, even when stdout isn't a terminal. |

Neither can change a `Finding`, an exit code, or a byte of generated file
content — only whether the human-facing text printed alongside them carries
color. `--format github` output never carries color regardless of either
variable. This preserves the original goal (CI and every agent harness get
identical, reproducible *results* from the repository contents alone) while
letting a human at a real terminal get styled output. See
[ADR 0011](../decisions/0011-build-time-go-dependencies.md).

## Access for `maat contracts update`

One narrow exception: `maat contracts update`, the only command that uses the
network ([ADR 0014](../decisions/0014-cross-repo-contracts-via-lockfile.md)),
reads **credentials and the GitHub API location** from the environment.
Credentials must never be committed to `.maat.yml`, and these variables only
decide whether a provider's manifest can be *reached* — never what is
validated or how. `sync` and `check` read none of them.

| Variable | Effect |
|----------|--------|
| `GITHUB_TOKEN`, `GH_TOKEN` | Bearer token for GitHub sources (first non-empty wins). Needed for private repositories; also raises GitHub's rate limit. |
| `MAAT_TOKEN` | Bearer token for `url` sources. GitHub tokens are never sent to `url` sources. |
| `GITHUB_API_URL` | GitHub API root, default `https://api.github.com`. GitHub Actions sets it automatically, including on GitHub Enterprise Server. |
| `HTTPS_PROXY` | Standard proxy setting, honored by the HTTP client. |

Tokens are never printed or written to the lockfile.
