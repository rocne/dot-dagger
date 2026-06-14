#!/bin/sh
set -e

export HOME=/home/e2e
export XDG_BIN_HOME=/home/e2e/bin
export XDG_DATA_HOME=/tmp/xdgdata
mkdir -p /home/e2e/bin /tmp/xdgdata

COMMON_ARGS="--files /fixture --dotd-env /fixture/env.yaml --env os=linux --env context=personal"

OUT=$(dotd list $COMMON_ARGS)
echo "$OUT" | grep -q "base"    || { printf 'FAIL: dotd list missing "base"\nOutput:\n%s\n'    "$OUT"; exit 1; }
echo "$OUT" | grep -q "path"    || { printf 'FAIL: dotd list missing "path"\nOutput:\n%s\n'    "$OUT"; exit 1; }
echo "$OUT" | grep -q "aliases" || { printf 'FAIL: dotd list missing "aliases"\nOutput:\n%s\n' "$OUT"; exit 1; }
echo "$OUT" | grep -q "linux"   || { printf 'FAIL: dotd list missing "linux"\nOutput:\n%s\n'   "$OUT"; exit 1; }

! echo "$OUT" | grep -q "macos" \
  || { printf 'FAIL: dotd list shows inactive "macos"\nOutput:\n%s\n' "$OUT"; exit 1; }
! echo "$OUT" | grep -q "work" \
  || { printf 'FAIL: dotd list shows inactive "work"\nOutput:\n%s\n' "$OUT"; exit 1; }

OUT_ALL=$(dotd list --inactive $COMMON_ARGS)
echo "$OUT_ALL" | grep -q "macos" \
  || { printf 'FAIL: dotd list --inactive missing "macos"\nOutput:\n%s\n' "$OUT_ALL"; exit 1; }
echo "$OUT_ALL" | grep -q "work" \
  || { printf 'FAIL: dotd list --inactive missing "work"\nOutput:\n%s\n' "$OUT_ALL";  exit 1; }

printf 'PASS: list test\n'
