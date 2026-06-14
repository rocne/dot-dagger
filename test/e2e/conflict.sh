#!/bin/sh
set -e

export HOME=/home/e2e
export XDG_BIN_HOME=/home/e2e/bin
export XDG_DATA_HOME=/tmp/xdgdata
mkdir -p /home/e2e/bin /tmp/xdgdata

printf 'existing content\n' > /home/e2e/.zshrc

APPLY_ARGS="--files /fixture --dotd-env /fixture/env.yaml --env os=linux --env context=personal"

dotd apply $APPLY_ARGS 2>/dev/null \
  && { printf 'FAIL: apply without --force should have failed on plain-file conflict\n'; exit 1; } \
  || true
test -L /home/e2e/.zshrc \
  && { printf 'FAIL: apply without --force must not replace plain file with symlink\n'; exit 1; } \
  || true

rm -f /home/e2e/.zshrc
printf 'existing content\n' > /home/e2e/.zshrc

dotd apply $APPLY_ARGS --force \
  || { printf 'FAIL: apply --force failed\n'; exit 1; }

test -L /home/e2e/.zshrc \
  || { printf 'FAIL: .zshrc is not a symlink after apply --force\n'; exit 1; }
TARGET=$(readlink /home/e2e/.zshrc)
[ "$TARGET" = "/fixture/conf/dot-zshrc" ] \
  || { printf 'FAIL: .zshrc symlink target wrong: %s\n' "$TARGET"; exit 1; }

printf 'PASS: conflict test\n'
