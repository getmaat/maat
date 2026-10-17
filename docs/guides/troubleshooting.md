---
title: Troubleshooting
status: current
summary: Common maat check failures and how to resolve them.
---

# Troubleshooting

## `check` reports **drift** on a generated file

The file was hand-edited or a `sync` was forgotten. Run:

```bash
maat sync
```

Then commit the regenerated files. Never edit inside a
`<!-- maat:begin -->` / `<!-- maat:end -->` block or a generated `.mdc`
file — put your prose outside the managed region.

## `check` reports **staleness** but the doc is actually current

In a git repository, staleness means a commit after the doc's last change
touched one of its `related_code` paths (see
[ADR 0012](../decisions/0012-git-aware-staleness.md)). Review the doc, and
update it in the same change as the code — a commit that touches the doc
clears the finding. Touching the file's timestamp no longer does.

If `check` also warns that **git history is shallow**, CI checked out only the
latest commit. Use `actions/checkout` with `fetch-depth: 0`.

Outside git (or with `check.staleness_source: mtime`), staleness is the old
mtime heuristic: the code file is newer than the doc. Re-save the reviewed doc
so its timestamp advances (`maat sync` does this).

If a path should never trigger staleness (generated code, vendored files), add
it to `check.ignore_code_paths` in `.maat.yml`.

## `check` reports **orphaned_code**

A `related_code` path no longer exists — the file was moved or deleted. Update
the doc's `related_code` front-matter to the new path, or remove the entry if
the code is gone.

## `check` reports a **broken_link**

An internal Markdown link points at a file that does not exist. Fix the path
(links are relative to the file they live in) or create the target. External
links (`http`, `mailto`, `#anchors`) are not checked.

## `check` exits `2` with "No docs/ directory"

Ma'at has not been initialized here, or `docs_dir` in `.maat.yml` points
somewhere else. Run `maat init` or fix the config.

## A doc fails the **frontmatter** check

It is missing a required key (`title`/`status` by default) or uses a `status`
outside the allowed set. See the
[front-matter reference](../reference/frontmatter.md).
