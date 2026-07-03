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

Staleness is an mtime heuristic (see
[ADR 0004](../decisions/0004-related-code-staleness.md)): the code file is
newer than the doc. If you have reviewed the doc and it is correct, re-save it
so its timestamp advances:

```bash
maat sync     # or just touch/re-save the doc file
```

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
