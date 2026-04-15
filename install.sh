#!/usr/bin/env sh
# install.sh — download and install the latest dotr release
#
# Usage:
#   ./install.sh
#   INSTALL_DIR=/usr/local/bin ./install.sh
#
# Requires: gh CLI (https://cli.github.com), authenticated with `gh auth login`
#
set -e

REPO="rocne/dot-dagger"
TOOL="dotr"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

# --- detect OS ---
os=$(uname -s)
case "$os" in
  Linux)  os="linux"  ;;
  Darwin) os="darwin" ;;
  *) printf 'error: unsupported OS: %s\n' "$os" >&2; exit 1 ;;
esac

# --- detect architecture ---
arch=$(uname -m)
case "$arch" in
  x86_64|amd64)  arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) printf 'error: unsupported architecture: %s\n' "$arch" >&2; exit 1 ;;
esac

printf 'platform: %s/%s\n' "$os" "$arch"

# --- require gh ---
if ! command -v gh >/dev/null 2>&1; then
  printf 'error: gh CLI not found\n' >&2
  printf '       install from https://cli.github.com then run: gh auth login\n' >&2
  exit 1
fi

# --- find latest dotr release ---
TAG=$(gh api "repos/$REPO/releases" \
  --jq '[.[] | select(.tag_name | startswith("dotr-"))][0].tag_name')

if [ -z "$TAG" ] || [ "$TAG" = "null" ]; then
  printf 'error: no dotr release found in %s\n' "$REPO" >&2
  exit 1
fi

# tag: dotr-v0.1.4  →  asset prefix: dotr_v0.1.4
ASSET_PREFIX=$(printf '%s' "$TAG" | sed 's/dotr-/dotr_/')
ASSET="${ASSET_PREFIX}_${os}_${arch}.tar.gz"

printf 'release: %s\n' "$TAG"
printf 'asset:   %s\n' "$ASSET"

# --- download to temp dir ---
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

printf 'downloading...\n'
gh release download "$TAG" --repo "$REPO" --pattern "$ASSET" --dir "$TMP"

# --- extract ---
tar -xzf "$TMP/$ASSET" -C "$TMP"

# --- install ---
mkdir -p "$INSTALL_DIR"
mv "$TMP/$TOOL" "$INSTALL_DIR/$TOOL"
chmod +x "$INSTALL_DIR/$TOOL"

printf 'installed: %s/%s\n' "$INSTALL_DIR" "$TOOL"
"$INSTALL_DIR/$TOOL" --version

# --- PATH reminder if needed ---
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    printf '\nNote: %s is not in your PATH. Add to your shell rc:\n' "$INSTALL_DIR"
    printf '  export PATH="%s:$PATH"\n' "$INSTALL_DIR"
    ;;
esac
