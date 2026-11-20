#!/usr/bin/env bash
# Runs the demo scenario with a real maat binary and saves every command's
# output, so the demo GIF never shows invented output.
#
#   lib/record-scenario.sh <maat-binary> <work-dir>
#
# Creates two repositories in <work-dir>: "ledger" provides a contract,
# "billing" consumes it through a local `path:` source (no network needed).
set -euo pipefail
MAAT=$1
WORK=$2
OUT=$WORK/out
mkdir -p "$OUT"
export GIT_AUTHOR_NAME=demo GIT_AUTHOR_EMAIL=demo@example.com
export GIT_COMMITTER_NAME=demo GIT_COMMITTER_EMAIL=demo@example.com
export NO_COLOR=1
git_init() { git init -q -b main 2>/dev/null || git init -q; }

# Provider: ledger publishes ledger-events 1.6.0.
mkdir -p "$WORK/ledger" && cd "$WORK/ledger" && git_init
"$MAAT" init . --name Ledger --summary "Ledger events." --agents= >/dev/null
mkdir -p docs/architecture/modules
cat > docs/architecture/modules/events.md <<'DOC'
---
title: Ledger events
status: current
summary: Events the ledger publishes.
provides:
  - id: ledger-events
    kind: event
    version: 1.6.0
---
# Ledger events
DOC
"$MAAT" sync . >/dev/null
git add -A && git commit -qm init

# Consumer: billing.
mkdir -p "$WORK/billing" && cd "$WORK/billing" && git_init
"$MAAT" init . --name Billing --summary "Invoices and payments." --agents=claude,copilot,cursor > "$OUT/init.txt" 2>&1
mkdir -p docs/architecture/modules internal/auth
printf 'package auth\n\nconst TokenTTL = 15 * time.Minute\n' > internal/auth/token.go
cat > docs/architecture/modules/auth.md <<'DOC'
---
title: Auth tokens
status: current
summary: How access tokens are issued.
related_code:
  - internal/auth/token.go
consumes:
  - ref: acme/ledger#ledger-events
    version: "~> 1.6"
---
Access tokens expire after 15 minutes.
DOC
printf '\ncontracts:\n  sources:\n    acme/ledger:\n      path: ../ledger\n' >> .maat.yml
cp docs/architecture/modules/auth.md "$OUT/auth.md"
"$MAAT" sync . >/dev/null
"$MAAT" contracts update . > "$OUT/update1.txt" 2>&1
git add -A && git commit -qm docs
"$MAAT" check . > "$OUT/check1.txt" 2>&1

# A teammate changes the code only.
sed -i.bak 's/15 \* time.Minute/60 * time.Minute/' internal/auth/token.go && rm internal/auth/token.go.bak
git diff --no-color > "$OUT/diff.txt"
git commit -qam "Tokens last an hour"
"$MAAT" check . > "$OUT/check2.txt" 2>&1 || true

# The ledger team ships ledger-events 2.0.0.
cd "$WORK/ledger"
sed -i.bak 's/version: 1.6.0/version: 2.0.0/' docs/architecture/modules/events.md && rm docs/architecture/modules/events.md.bak
"$MAAT" sync . >/dev/null && git commit -qam "ledger-events 2.0.0"
cd "$WORK/billing"
"$MAAT" contracts update --check . > "$OUT/update-check.txt" 2>&1 || true
"$MAAT" contracts update . > "$OUT/update2.txt" 2>&1
"$MAAT" check . > "$OUT/check3.txt" 2>&1 || true

# The init output names the absolute work directory; record it so the
# storyboard can show a neutral path instead.
echo "$WORK/billing" > "$OUT/billing-path.txt"
