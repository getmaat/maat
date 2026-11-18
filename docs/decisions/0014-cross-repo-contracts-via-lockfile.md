---
title: 0014 Resolve cross-repo contracts through a committed lockfile
status: draft
summary: An explicit, opt-in command fetches the manifests of the repositories a repo consumes into a committed lockfile; check validates cross-repo refs against that lockfile offline, so sync and check stay deterministic.
---

# 0014. Resolve cross-repo contracts through a committed lockfile

- **Status:** proposed
- **Date:** 2026-11-18
- **Relates to:** [0013 Typed contract edges and a generated manifest](0013-contract-edges-and-manifest.md), [0011 Build-time Go dependencies are permitted; runtime dependencies are not](0011-build-time-go-dependencies.md), [0007 Agent skills as managed artifacts](0007-agent-skills-as-managed-artifacts.md)

## Context

[ADR 0013](0013-contract-edges-and-manifest.md) lets a doc declare
`consumes: [{ref: owner/repo#id, version: "~> 1.4"}]` and records it in
`.maat/manifest.json`, but leaves cross-repo refs **unresolved**: nothing
checks that `owner/repo` provides `id`, or that its version satisfies the
constraint. That check is the point of the feature — it is what catches a
provider's breaking change before a consumer finds out in production, and
what gives an agent in the consumer repo trustworthy knowledge of the other
side.

Resolving a ref means reading another repository's manifest, which lives
somewhere else: usually another Git host repository, sometimes a sibling
checkout. That is a **network call**, and Ma'at has so far held a firm line
against them: `sync` and `check` are deterministic and offline (ADRs 0007,
0008), and the coding conventions rule out runtime network calls (ADR 0011).
Those properties are why `check` is a trustworthy CI gate — the same commit
always yields the same verdict.

So the question is not only *where* manifests come from, but *when* Ma'at may
reach out for them without making `check` non-deterministic.

## Decision

Follow the lockfile pattern of package managers (`go.sum`,
`package-lock.json`): fetching is an **explicit, opt-in command** that writes
a **committed snapshot**; everything else reads the snapshot offline.

### 1. Sources in `.maat.yml`

A repository lists where to find the manifests of the repositories it
consumes:

```yaml
contracts:
  sources:
    getmaat/ledger:                    # no value: GitHub, default branch
    getmaat/billing:
      ref: release                     # optional branch, tag or commit
    acme/payments:
      url: https://git.acme.example/api/v4/projects/acme%2Fpayments/repository/files/.maat%2Fmanifest.json/raw?ref={ref}
      ref: main
```

- A source with no `url` is fetched from GitHub
  (`api.github.com/repos/{repo}/contents/.maat/manifest.json?ref={ref}`), the
  common case, with the commit resolved to a SHA.
- `url` is a template for any other host; `{repo}` and `{ref}` are filled in.
  Ma'at does not need to know GitLab, Gitea or Bitbucket APIs.
- Authentication comes from the environment, never from `.maat.yml`:
  `GITHUB_TOKEN` / `GH_TOKEN` for GitHub, `MAAT_TOKEN` (sent as a bearer
  token) for `url` sources. Public repositories need none.
- A cross-repo `consumes` ref whose repository has **no** source entry is a
  `contract_schema` finding, so a forgotten source is caught instead of the
  ref silently going unchecked.

A source with no value (`getmaat/ledger:`) uses all defaults. Any value that
is neither empty nor a mapping is a config error; in particular `{}` must be
rejected with a hint, because the YAML subset reads it as the string `"{}"`.

### 2. `maat contracts update` writes `.maat/contracts.lock.json`

A new command fetches every source's manifest and writes a lockfile:

```json
{
  "schema": 1,
  "sources": {
    "getmaat/ledger": {
      "ref": "main",
      "commit": "3f9c2e1…",
      "manifest_schema": 1,
      "provides": [
        {"id": "ledger-events", "kind": "event", "version": "1.6.0",
         "doc": "docs/architecture/modules/events.md"}
      ]
    }
  }
}
```

- It records **only what this repository consumes** from each source, not the
  whole remote manifest, so the lockfile stays small and its diff shows
  exactly the contracts that changed.
- It records the resolved commit when the host provides one, so the snapshot
  can be traced back and reproduced.
- It contains no timestamps, so an update with no remote changes is a no-op
  diff.
- `maat contracts update --check` fetches and **compares** without writing,
  exiting non-zero when the lockfile is out of date — for a scheduled CI job
  (below).
- This is the **only** command that touches the network. It is never run
  implicitly by `sync`, `check` or `init`.

### 3. `check` validates cross-repo refs against the lockfile, offline

The ADR 0013 rules extend to cross-repo refs, reading the lockfile:

| Rule | Cross-repo meaning |
|------|--------------------|
| `contract_unresolved` | the source's locked `provides` has no such `id`, or the source is missing from the lockfile (run `maat contracts update`) |
| `contract_version` | the locked version does not satisfy the constraint, or the provider declares no version |
| `contract_schema` | a consumed repository has no `contracts.sources` entry |

`check` also flags a lockfile that no longer matches the sources and refs
declared (a source added or removed, a ref no longer consumed) as `drift`,
with "run `maat contracts update`" as the remedy — the same pattern as other
generated files, except that regenerating it needs the network.

### 4. Staying current is a scheduled job, not a `check` side effect

An offline `check` cannot know that a provider released a breaking change
after the lockfile was written. That is the trade-off of this design, and it
is made explicitly: freshness comes from a **scheduled CI job** running
`maat contracts update --check` (daily, say), which fails — or opens a PR with
the updated lockfile — when a provider changed. The scaffolded workflow and
the reusable workflow gain an optional job for this.

The pull-request gate stays deterministic: the same commit always passes or
fails the same way, and a provider's change shows up as a reviewable lockfile
diff in the consumer, not as a red build on an unrelated PR.

### 5. Local overrides for development

A source may also set `path: ../ledger` to read a sibling checkout's
`.maat/manifest.json` instead of fetching, for developing two repositories
side by side. `path` sources never require the network; the lockfile records
them without a commit.

## Consequences

- Consumers find out about provider breaking changes — the payoff of ADR 0013.
- `sync` and `check` remain deterministic and offline; the PR gate is
  unaffected by what other repositories do in the meantime.
- **Ma'at gains its first network client**, confined to one command, using
  only the Go standard library (`net/http`). The coding convention in the
  development guide changes from "no network call" to "no network call
  outside `maat contracts update`". ADR 0011 is unaffected (no new
  dependency).
- Agents in a consumer repository get the providers' contracts from the
  lockfile, offline. A later change can surface them in `llms.txt` as well.
- Teams must add a scheduled job to stay current; without it, the lockfile is
  only as fresh as its last manual update. The scaffolded workflow makes that
  the default.
- Token handling enters the codebase. Tokens are read from the environment,
  only sent to the configured host, and never written to the lockfile or
  printed.

## Alternatives considered

- **Fetch during `check`.** Simplest to use, and always current. Rejected:
  `check` would need network and credentials in every CI run and on every
  laptop, could fail on a flaky connection or rate limit, and the same commit
  could pass today and fail tomorrow. That breaks the property that makes
  `check` a trustworthy gate.
- **An org index repository** that collects every repo's manifest (pushed by
  each repo's CI). Gives one place to query, and the *reverse* view ("who
  consumes my API?") for free. Rejected as the first step: it needs shared
  infrastructure, write tokens across repositories and an owner. It stays
  compatible: an index can later be just another `url` source, and is the
  natural home for the reverse view.
- **Git submodules or vendored copies of providers' docs.** Offline and
  pinned, but heavyweight, copies far more than the contracts, and submodules
  are widely disliked. The lockfile is a minimal, purpose-built vendoring.
- **Shelling out to `gh` or `git archive`.** Avoids writing an HTTP client,
  but adds a runtime dependency on external tools (against ADR 0011's spirit)
  and still needs credentials. The standard library's HTTP client is enough.
- **GitHub only, no `url` template.** Less to build, but Ma'at is
  host-agnostic everywhere else; a URL template costs little and keeps GitLab
  and self-hosted users in.

## Open questions for review

1. Should the scheduled freshness job **open a pull request** with the updated
   lockfile, or only fail and leave the update to a human? Opening PRs needs a
   write token and more workflow code.
2. Should a provider's change to a **consumed contract's doc or spec** (not
   just its version) also show up — for example by recording a content hash
   in the lockfile — or is the version the right level of signal?
3. Is the **reverse view** ("who consumes my contracts?") needed for v0.9, or
   can it wait for an org index (see alternatives)?
