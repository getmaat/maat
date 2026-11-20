#!/usr/bin/env bash
# Regenerates the README visuals into .github/assets/:
#   banner.webp          README hero banner
#   social-preview.png   GitHub social preview (upload in repo settings)
#   demo.gif             terminal demo, built from real command output
#
# Usage: scripts/readme-assets/build.sh [banner|demo|all]   (default: all)
# Maintainer tooling only; Ma'at itself does not use any of this.
set -euo pipefail
HERE=$(cd "$(dirname "$0")" && pwd)
REPO=$(cd "$HERE/../.." && pwd)
ASSETS=$REPO/.github/assets
WHAT=${1:-all}

for tool in node python3 ffmpeg go git; do
  command -v "$tool" >/dev/null || { echo "missing required tool: $tool" >&2; exit 1; }
done
python3 -c 'import PIL, numpy' 2>/dev/null || { echo "missing Python packages: pip install pillow numpy" >&2; exit 1; }
[ -d "$HERE/node_modules" ] || (cd "$HERE" && npm install --silent)
# Playwright needs a Chromium; skip if one is already provided (PLAYWRIGHT_BROWSERS_PATH).
[ -n "${PLAYWRIGHT_BROWSERS_PATH:-}" ] || (cd "$HERE" && npx playwright install chromium >/dev/null)

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
export NODE_PATH=$HERE/node_modules

# Shared inputs: templates, fonts, transparent logo.
cp "$HERE"/templates/* "$WORK"/
cp "$HERE"/node_modules/@fontsource/jetbrains-mono/files/jetbrains-mono-latin-{400,700}-normal.woff2 "$WORK"/
cp "$HERE"/node_modules/@fontsource-variable/inter/files/inter-latin-opsz-normal.woff2 "$WORK"/
python3 "$HERE/lib/logo-alpha.py" "$ASSETS/maat-logo.webp" "$WORK/maat-logo-alpha.png"

if [ "$WHAT" = banner ] || [ "$WHAT" = all ]; then
  echo "Rendering banner and social preview..."
  (cd "$HERE" && node lib/render-card.mjs "$WORK/banner.html" "$WORK/banner.png" 1280 400)
  (cd "$HERE" && node lib/render-card.mjs "$WORK/social.html" "$WORK/social.png" 1280 640)
  python3 - "$WORK" "$ASSETS" <<'PY'
import sys
from PIL import Image
work, assets = sys.argv[1], sys.argv[2]
Image.open(f'{work}/banner.png').save(f'{assets}/banner.webp', 'WEBP', quality=92, method=6, alpha_quality=100)
Image.open(f'{work}/social.png').convert('RGB').resize((1280, 640), Image.LANCZOS).save(f'{assets}/social-preview.png', optimize=True)
PY
  echo "  wrote banner.webp, social-preview.png"
fi

if [ "$WHAT" = demo ] || [ "$WHAT" = all ]; then
  echo "Recording the demo scenario with maat built from source..."
  (cd "$REPO" && go build -o "$WORK/maat" .)
  "$HERE/lib/record-scenario.sh" "$WORK/maat" "$WORK/scenario"
  cp "$WORK"/terminal.html "$WORK"/*.woff2 "$WORK"/maat-logo-alpha.png "$WORK/scenario"/
  python3 "$HERE/lib/story.py" "$WORK/scenario"
  echo "Rendering frames..."
  (cd "$HERE" && node lib/capture-demo.mjs "$WORK/scenario" 15)
  echo "Encoding GIF..."
  ffmpeg -loglevel error -y -framerate 15 -i "$WORK/scenario/frames/%05d.png" \
    -vf "palettegen=stats_mode=diff:max_colors=256" "$WORK/palette.png"
  ffmpeg -loglevel error -y -framerate 15 -i "$WORK/scenario/frames/%05d.png" -i "$WORK/palette.png" \
    -lavfi "paletteuse=dither=bayer:bayer_scale=4:diff_mode=rectangle" -loop 0 "$ASSETS/demo.gif"
  echo "  wrote demo.gif"
  echo
  echo "Remember: the 'What it catches' output block in README.md should match"
  echo "the demo's final run:"
  sed 's/^/  | /' "$WORK/scenario/out/check3.txt"
fi
