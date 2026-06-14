#!/bin/sh
set -e

export HOME=/home/e2e
export XDG_BIN_HOME=/home/e2e/bin
export XDG_DATA_HOME=/tmp/xdgdata
mkdir -p /home/e2e/bin /tmp/xdgdata

dotd apply \
  --files /fixture \
  --dotd-env /fixture/env.yaml \
  --env os=linux \
  --env context=personal

test -L /home/e2e/bin/dot-dagger/hello \
  || { printf 'FAIL: bin/hello symlink missing from bin-dir\n'; exit 1; }

TARGET=$(readlink /home/e2e/bin/dot-dagger/hello)
[ "$TARGET" = "/fixture/bin/hello" ] \
  || { printf 'FAIL: bin/hello symlink target wrong: %s\n' "$TARGET"; exit 1; }

test -x /home/e2e/bin/dot-dagger/hello \
  || { printf 'FAIL: bin/hello is not executable via symlink\n'; exit 1; }

printf 'PASS: bin test\n'
