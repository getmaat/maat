---
title: Terminal presentation module
status: current
summary: TTY/color detection, Lip Gloss styling, and the Huh init wizard layered on top of the plain-text output every command already produces.
related_code:
  - internal/maat/tty.go
  - internal/maat/style.go
  - internal/maat/wizard.go
---

# Terminal presentation

## Responsibility

Add optional, TTY-gated terminal UX — colored output and an interactive
`init` prompt — without changing what any command prints when its output
isn't an interactive terminal. CI, scripts, `--format github`, and every
existing test must see exactly the plain text they saw before this module
existed (see [ADR 0011](../../decisions/0011-build-time-go-dependencies.md)).

## Key files

- `internal/maat/tty.go` — `isInteractiveTerminal(stdout io.Writer) bool`, a
  var-func gating the `init` wizard. True only when stdout is a real
  `*os.File` and both stdout and stdin pass `isatty.IsTerminal`.
- `internal/maat/style.go` — `isColorEnabled(stdout io.Writer) bool`, a
  separate var-func gating ANSI styling (`NO_COLOR` forces it off,
  `CLICOLOR_FORCE` forces it on, otherwise it follows
  `isInteractiveTerminal`); Lip Gloss style definitions; `styled*Line`
  functions that wrap the exact literal text `cli.go` already prints in
  ANSI escapes — never changing spacing or wording.
- `internal/maat/wizard.go` — the Huh form (`runInitWizard`, a var-func
  seam) that prompts for `init`'s project name, summary, and agent adapter
  selection (a `huh.MultiSelect` over `adapterOrder`) when none of
  `--name`/`--summary`/`--agents` were given and `isInteractiveTerminal` is
  true.

## Interfaces / contracts

- Both detection functions are package-level `var`s, not plain funcs, so
  tests can override them to force either code path deterministically
  without a real terminal or PTY.
- Every styled render function has a plain counterpart already used by
  `cli.go`; the styled one must reproduce it byte-for-byte once ANSI escapes
  are stripped. `check.go`'s `Finding`/`Finding.String()` and `emitGitHub`
  are never touched — `styledFindingLine` in `style.go` is a parallel
  function, not a rewrite, and `--format github` is never routed through
  color regardless of TTY/env state.
- `runInitWizard` returns `wizardResult{name, summary string; agents []string;
  ok bool}`; `ok == false` with a `nil` error means the user aborted the form
  (`huh.ErrUserAborted`) — `cmdInit` maps that to exit code `130`, not an
  error. The multi-select's initial checked state is seeded from the
  caller-supplied `defaultAgents` (the repo's current `adapters:`, or every
  adapter for a fresh repo) — see [configuration reference:
  adapters](../../reference/configuration.md#adapters).

## Gotchas

- Because every test in `cli_test.go` drives `Main` with a `*bytes.Buffer`/
  `*strings.Builder`, `isInteractiveTerminal`'s `stdout.(*os.File)` assertion
  always fails there — the wizard and styled-output branches are
  structurally unreachable from the existing suite unless a test explicitly
  overrides the seam. This is intentional, not a coverage gap to close.
- Lip Gloss's default renderer auto-detects color support from the real
  process's stdout file descriptor, which would silently disable styling in
  any non-terminal process (including tests that override `isColorEnabled`
  to force color on). `style.go` builds its styles from a dedicated
  `lipgloss.Renderer` pinned to `termenv.ANSI`, so `isColorEnabled` is the
  only thing deciding whether color appears — Lip Gloss's own detection
  never gets a second vote.
- `huh.Form.Run()`/Bubble Tea can panic instead of returning an error on a
  malformed terminal (observed: a zero-width window size crashing a
  `bubbles/textinput` render). `wizard.go`'s `runForm` wraps the call in a
  `recover()` and treats any panic like a wizard that couldn't start —
  falling back to the same defaults `init` uses non-interactively — so a
  misbehaving terminal degrades `init`'s UX, it never crashes the command.
