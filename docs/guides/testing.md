---
title: Testing guide
status: current
summary: How to run and write Ma'at's tests, and what CI enforces.
---

# Testing guide

## Running the tests

```bash
go test ./...                                  # whole suite
go test ./internal/maat/                    # one package
go test ./internal/maat/ -run TestCheckStrictPromotesStaleness   # one test
```

The tests exercise the CLI end to end against temporary directories: they run
`init`, `sync`, and `check`, and assert on findings and exit codes.

## What CI enforces

The [`Ma'at` workflow](../../.github/workflows/maat.yml) runs on every pull
request and push to the main branch. It builds the binary and runs:

```bash
go build -o maat .
./maat check --format github
```

A merge is blocked if `check` reports any error-severity finding — stale docs,
broken internal links, missing `related_code` targets, or drifted generated
files. Fix them by updating the relevant doc and running `maat sync`.

## Writing tests

- Tests live in `internal/maat/` (e.g. `cli_test.go`) and use Go's `testing`
  package with `t.TempDir()` for isolation.
- Prefer black-box tests that invoke the CLI through `maat.Main([...])` and
  assert on the return code, over testing internal functions — this keeps the
  CLI contract (documented in the [CLI reference](../reference/cli.md)) covered.
- When you add a validation rule, add a test that proves it both *fires* on a
  bad fixture and *stays quiet* on a good one.
