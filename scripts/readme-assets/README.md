# README assets (maintainer tooling)

**You don't need anything in this folder to use or develop Ma'at.** It only regenerates the images shown in the project README:

| File in `.github/assets/` | What it is |
| --- | --- |
| `banner.webp` | The README hero banner |
| `social-preview.png` | The image GitHub shows when the repository link is shared (upload it under Settings → General → Social preview) |
| `demo.gif` | The terminal demo in "What it catches" |

## When to run it

- **Ma'at's output changed** (a message, a new finding, `init`'s file list): the demo shows real output and needs regenerating.
- **The pitch changed**: edit the text in `templates/banner.html` and `templates/social.html`.

## How to run it

```bash
scripts/readme-assets/build.sh          # everything
scripts/readme-assets/build.sh banner   # banner + social preview only
scripts/readme-assets/build.sh demo     # demo GIF only
```

Requirements: Go, Node.js 18+, Python 3 with `pillow` and `numpy`, `ffmpeg`, `git`. The script installs its pinned npm packages (Playwright and the fonts) into this folder on first run, plus Playwright's Chromium.

After regenerating the demo, the script prints the demo's final `maat check` output; the "What it catches" block in the README should match it.

## How it works

- **No invented output.** `lib/record-scenario.sh` builds `maat` from this repository and runs the demo story for real in a temporary directory: a `billing` repo consuming a contract from a sibling `ledger` repo (through a local `path:` source, so no network). `lib/story.py` turns the recorded output into a storyboard; the only edit is showing the temporary path as `~/acme/billing`.
- **Deterministic rendering.** `templates/terminal.html` maps a point in time to the screen state; `lib/capture-demo.mjs` renders it frame by frame in headless Chromium, and `ffmpeg` encodes the GIF. The banner and social preview are HTML/CSS (`templates/`) screenshotted at 2×.
- **Brand:** background `#00060b`, teal `#03cdb4`, gold `#f2b33d`, pink `#e0559a`; Inter (headlines) and JetBrains Mono (terminal, ligatures off so `~>` and `--` stay literal). `lib/logo-alpha.py` makes the logo's background transparent so it sits cleanly on gradients.
