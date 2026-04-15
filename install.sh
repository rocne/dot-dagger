#!/usr/bin/env sh
# install.sh — download and install a dotr suite tool
#
# One-liner (private repo — uses gh for auth):
#   curl -fsSL -H "Authorization: Bearer $(gh auth token)" \
#     https://raw.githubusercontent.com/rocne/dot-dagger/main/install.sh | sh
#
# With args (pass after --):
#   curl -fsSL -H "Authorization: Bearer $(gh auth token)" \
#     https://raw.githubusercontent.com/rocne/dot-dagger/main/install.sh | sh -s -- dotl --version v0.1.4
#
# Local usage:
#   ./install.sh [tool] [--version v0.1.4] [--os linux|darwin] [--arch amd64|arm64] [--dir path]
#
# tool     — one of: dotr dotd dote dotl dotp  (default: dotr)
# --version  specific version to install       (default: latest)
# --os       override OS detection
# --arch     override architecture detection
# --dir      override install directory        (default: ~/.local/bin)
#
# Requires: gh CLI (https://cli.github.com), authenticated with `gh auth login`
#
set -e

REPO="rocne/dot-dagger"
VALID_TOOLS="dotr dotd dote dotl dotp"

# --- defaults ---
TOOL="dotr"
VERSION=""
OS=""
ARCH=""
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

# --- parse args ---
while [ $# -gt 0 ]; do
  case "$1" in
    --version) VERSION="$2"; shift 2 ;;
    --os)      OS="$2";      shift 2 ;;
    --arch)    ARCH="$2";    shift 2 ;;
    --dir)     INSTALL_DIR="$2"; shift 2 ;;
    --help|-h)
      sed -n '2,12p' "$0" | sed 's/^# \?//'
      exit 0
      ;;
    --*) printf 'error: unknown flag: %s\n' "$1" >&2; exit 1 ;;
    *)
      TOOL="$1"
      shift
      ;;
  esac
done

# --- validate tool ---
valid=0
for t in $VALID_TOOLS; do
  [ "$TOOL" = "$t" ] && valid=1 && break
done
if [ "$valid" = "0" ]; then
  printf 'error: unknown tool %q. Valid tools: %s\n' "$TOOL" "$VALID_TOOLS" >&2
  exit 1
fi

# --- detect OS (unless overridden) ---
if [ -z "$OS" ]; then
  OS=$(uname -s)
  case "$OS" in
    Linux)  OS="linux"  ;;
    Darwin) OS="darwin" ;;
    *) printf 'error: unsupported OS: %s (use --os to override)\n' "$OS" >&2; exit 1 ;;
  esac
fi

# --- detect arch (unless overridden) ---
if [ -z "$ARCH" ]; then
  ARCH=$(uname -m)
  case "$ARCH" in
    x86_64|amd64)  ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *) printf 'error: unsupported architecture: %s (use --arch to override)\n' "$ARCH" >&2; exit 1 ;;
  esac
fi

printf 'tool:     %s\n' "$TOOL"
printf 'platform: %s/%s\n' "$OS" "$ARCH"

# --- require gh ---
if ! command -v gh >/dev/null 2>&1; then
  printf 'error: gh CLI not found\n' >&2
  printf '       install from https://cli.github.com then run: gh auth login\n' >&2
  exit 1
fi

# --- resolve version to tag ---
if [ -n "$VERSION" ]; then
  # normalize: strip leading 'v' then re-add, so both 'v0.1.4' and '0.1.4' work
  VERSION=$(printf '%s' "$VERSION" | sed 's/^v//')
  TAG="${TOOL}-v${VERSION}"
else
  TAG=$(gh api "repos/$REPO/releases" \
    --jq "[.[] | select(.tag_name | startswith(\"${TOOL}-\"))][0].tag_name")

  if [ -z "$TAG" ] || [ "$TAG" = "null" ]; then
    printf 'error: no %s release found in %s\n' "$TOOL" "$REPO" >&2
    exit 1
  fi
fi

printf 'release:  %s\n' "$TAG"

# tag: dotr-v0.1.4  →  asset: dotr_v0.1.4_linux_amd64.tar.gz
ASSET_PREFIX=$(printf '%s' "$TAG" | sed "s/${TOOL}-/${TOOL}_/")
ASSET="${ASSET_PREFIX}_${OS}_${ARCH}.tar.gz"
printf 'asset:    %s\n' "$ASSET"

# --- download ---
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

printf 'downloading...\n'
gh release download "$TAG" --repo "$REPO" --pattern "$ASSET" --dir "$TMP"

# --- extract and install ---
tar -xzf "$TMP/$ASSET" -C "$TMP"
mkdir -p "$INSTALL_DIR"
mv "$TMP/$TOOL" "$INSTALL_DIR/$TOOL"
chmod +x "$INSTALL_DIR/$TOOL"

printf 'installed: %s/%s\n' "$INSTALL_DIR" "$TOOL"
"$INSTALL_DIR/$TOOL" --version

# --- PATH reminder ---
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    printf '\nNote: %s is not in your PATH. Add to your shell rc:\n' "$INSTALL_DIR"
    printf '  export PATH="%s:$PATH"\n' "$INSTALL_DIR"
    ;;
esac
