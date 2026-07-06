#!/bin/sh
# Ma'at installer — fetch a released static binary onto any macOS/Linux box or
# CI runner with no Go toolchain. This is the universal, language-agnostic
# distribution primitive that the GitHub Action and CI template build on
# (see ADR 0006 and docs/guides/deployment.md).
#
#   curl -sSf https://raw.githubusercontent.com/getmaat/maat/main/scripts/install.sh | sh
#
# Options are read from the environment so the one-liner stays clean:
#   MAAT_VERSION   version to install, with or without leading 'v'
#                  (default: latest release). Pin this in CI for reproducible
#                  runs — it should match the `maat_version` constraint in your
#                  repo's .maat.yml.
#   MAAT_INSTALL_DIR  install directory (default: /usr/local/bin if writable,
#                     else $HOME/.local/bin).
#   MAAT_NO_VERIFY    set to any value to skip sha256 checksum verification
#                     (NOT recommended; verification is on by default).
#
# The script is POSIX sh (no bashisms) and depends only on tools present on a
# stock macOS or Linux CI image: curl or wget, tar, and sha256sum or shasum.
set -eu

REPO="getmaat/maat"
BINARY="maat"

# --------------------------------------------------------------------------- #
# small helpers
# --------------------------------------------------------------------------- #
info() { printf 'maat-install: %s\n' "$1" >&2; }
err()  { printf 'maat-install: error: %s\n' "$1" >&2; exit 1; }

have() { command -v "$1" >/dev/null 2>&1; }

# download URL to stdout, following redirects, failing on HTTP errors.
fetch() {
  if have curl; then
    curl -fsSL "$1"
  elif have wget; then
    wget -qO- "$1"
  else
    err "need curl or wget to download"
  fi
}

# download URL to a file.
fetch_to() {
  if have curl; then
    curl -fsSL -o "$2" "$1"
  elif have wget; then
    wget -qO "$2" "$1"
  else
    err "need curl or wget to download"
  fi
}

# --------------------------------------------------------------------------- #
# platform detection — must match GoReleaser's archive name_template
# (maat_<version>_<os>_<arch>.tar.gz), see .goreleaser.yaml
# --------------------------------------------------------------------------- #
detect_os() {
  os="$(uname -s)"
  case "$os" in
    Darwin) echo "darwin" ;;
    Linux)  echo "linux" ;;
    *) err "unsupported OS '$os' — use 'go install github.com/${REPO}@latest', or download a release archive manually from https://github.com/${REPO}/releases" ;;
  esac
}

detect_arch() {
  arch="$(uname -m)"
  case "$arch" in
    x86_64 | amd64) echo "amd64" ;;
    arm64 | aarch64) echo "arm64" ;;
    *) err "unsupported architecture '$arch' — download a release archive manually from https://github.com/${REPO}/releases" ;;
  esac
}

# resolve the latest release tag via the GitHub API when no version is pinned.
latest_version() {
  # The API returns "tag_name": "vX.Y.Z"; extract without needing jq.
  fetch "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' \
    | head -n1 \
    | sed 's/.*"tag_name" *: *"\([^"]*\)".*/\1/'
}

# verify sha256 of $1 equals $2 using whichever tool is available.
verify_sha256() {
  file="$1"
  want="$2"
  if have sha256sum; then
    got="$(sha256sum "$file" | awk '{print $1}')"
  elif have shasum; then
    got="$(shasum -a 256 "$file" | awk '{print $1}')"
  else
    info "no sha256 tool found — skipping checksum verification"
    return 0
  fi
  if [ "$got" != "$want" ]; then
    err "checksum mismatch for $(basename "$file"): expected $want, got $got"
  fi
}

# --------------------------------------------------------------------------- #
# main
# --------------------------------------------------------------------------- #
OS="$(detect_os)"
ARCH="$(detect_arch)"

VERSION="${MAAT_VERSION:-}"
if [ -z "$VERSION" ]; then
  info "resolving latest release..."
  VERSION="$(latest_version)"
  [ -n "$VERSION" ] || err "could not resolve the latest release tag from the GitHub API"
fi
# Normalize: tag is vX.Y.Z, archive names use the bare X.Y.Z.
TAG="$VERSION"
case "$TAG" in v*) : ;; *) TAG="v$TAG" ;; esac
VER="${TAG#v}"

ARCHIVE="${BINARY}_${VER}_${OS}_${ARCH}.tar.gz"
BASE_URL="https://github.com/${REPO}/releases/download/${TAG}"

TMP="$(mktemp -d 2>/dev/null || mktemp -d -t maat)"
trap 'rm -rf "$TMP"' EXIT INT TERM

info "downloading ${ARCHIVE} (${TAG})..."
fetch_to "${BASE_URL}/${ARCHIVE}" "${TMP}/${ARCHIVE}" \
  || err "download failed — is ${TAG} a published release for ${OS}/${ARCH}? See https://github.com/${REPO}/releases"

if [ -z "${MAAT_NO_VERIFY:-}" ]; then
  info "verifying checksum..."
  fetch_to "${BASE_URL}/checksums.txt" "${TMP}/checksums.txt" \
    || err "could not download checksums.txt (set MAAT_NO_VERIFY=1 to skip verification)"
  want="$(grep " ${ARCHIVE}\$" "${TMP}/checksums.txt" | awk '{print $1}')"
  [ -n "$want" ] || err "no checksum entry for ${ARCHIVE} in checksums.txt"
  verify_sha256 "${TMP}/${ARCHIVE}" "$want"
fi

info "extracting..."
tar -xzf "${TMP}/${ARCHIVE}" -C "$TMP" \
  || err "failed to extract ${ARCHIVE}"
[ -f "${TMP}/${BINARY}" ] || err "archive did not contain a '${BINARY}' binary"
chmod +x "${TMP}/${BINARY}"

# Choose an install dir: prefer a system bin if writable, else a user bin.
DEST="${MAAT_INSTALL_DIR:-}"
if [ -z "$DEST" ]; then
  if [ -w /usr/local/bin ] 2>/dev/null; then
    DEST="/usr/local/bin"
  else
    DEST="${HOME}/.local/bin"
  fi
fi
mkdir -p "$DEST" || err "could not create install dir $DEST"

if mv "${TMP}/${BINARY}" "${DEST}/${BINARY}" 2>/dev/null; then
  :
elif have sudo && [ -t 0 ]; then
  info "elevating with sudo to write ${DEST}..."
  sudo mv "${TMP}/${BINARY}" "${DEST}/${BINARY}"
else
  err "could not write to ${DEST}; set MAAT_INSTALL_DIR to a writable path"
fi

info "installed ${BINARY} ${VER} to ${DEST}/${BINARY}"

# Nudge if the install dir is not on PATH (common for ~/.local/bin).
case ":${PATH}:" in
  *":${DEST}:"*) : ;;
  *) info "note: ${DEST} is not on your PATH — add it, e.g.  export PATH=\"${DEST}:\$PATH\"" ;;
esac

# Confirm it runs (best-effort; PATH may not include DEST yet).
if [ -x "${DEST}/${BINARY}" ]; then
  "${DEST}/${BINARY}" --version >&2 || true
fi
