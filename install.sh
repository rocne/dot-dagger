#!/usr/bin/env sh
# install.sh — download and install dotd
#
# One-liner:
#   curl -fsSL https://raw.githubusercontent.com/rocne/dot-dagger/main/install.sh | sh
#
# With args (pass after --):
#   curl -fsSL https://raw.githubusercontent.com/rocne/dot-dagger/main/install.sh | sh -s -- --version v0.2.0
#
# Local usage:
#   ./install.sh [--version v0.1.4] [--os linux|darwin] [--arch amd64|arm64] [--dir path] [--dry-run]
#
# --version    specific version to install  (default: latest)  e.g. v0.2.0
# --os         override OS detection
# --arch       override architecture detection
# --dir        override install directory   (default: ~/.local/bin)
# --dry-run    print what would be done, then exit without installing
#
# Requires: gh CLI (https://cli.github.com)
#
set -e

REPO="rocne/dot-dagger"
VALID_TOOLS="dotd"

# --- defaults ---
TOOL="dotd"
VERSION=""
OS=""
ARCH=""
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
DRY_RUN=0

# --- parse args ---
while [ $# -gt 0 ]; do
  case "$1" in
    --version)  VERSION="$2"; shift 2 ;;
    --os)       OS="$2";      shift 2 ;;
    --arch)     ARCH="$2";    shift 2 ;;
    --dir)      INSTALL_DIR="$2"; shift 2 ;;
    --dry-run)  DRY_RUN=1; shift ;;
    --help|-h)
      sed -n '2,14p' "$0" | sed 's/^# \?//'
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
  TAG="v${VERSION}"
else
  TAG=$(gh api "repos/$REPO/releases" \
    --jq "[.[] | select(.tag_name | startswith(\"v\"))][0].tag_name")

  if [ -z "$TAG" ] || [ "$TAG" = "null" ]; then
    printf 'error: no %s release found in %s\n' "$TOOL" "$REPO" >&2
    exit 1
  fi
fi

printf 'release:  %s\n' "$TAG"

# tag: v0.2.0  →  asset: dotd_v0.2.0_linux_amd64.tar.gz
ASSET_PREFIX="${TOOL}_${TAG}"
ASSET="${ASSET_PREFIX}_${OS}_${ARCH}.tar.gz"
CHECKSUMS="${ASSET_PREFIX}_checksums.txt"
printf 'asset:    %s\n' "$ASSET"

if [ "$DRY_RUN" = "1" ]; then
  printf 'install:  %s/%s\n' "$INSTALL_DIR" "$TOOL"
  printf '(dry-run: no changes made)\n'
  exit 0
fi

# --- download ---
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

printf 'downloading...\n'
gh release download "$TAG" --repo "$REPO" --pattern "$ASSET" --pattern "$CHECKSUMS" --dir "$TMP"

# --- verify checksum ---
printf 'verifying checksum...\n'
if command -v sha256sum >/dev/null 2>&1; then
  # filter to just this asset's line to avoid failures on missing files
  grep " ${ASSET}$" "$TMP/$CHECKSUMS" | (cd "$TMP" && sha256sum -c -)
elif command -v shasum >/dev/null 2>&1; then
  grep " ${ASSET}$" "$TMP/$CHECKSUMS" | (cd "$TMP" && shasum -a 256 -c -)
else
  printf 'warning: no sha256sum or shasum found — skipping checksum verification\n' >&2
fi

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
