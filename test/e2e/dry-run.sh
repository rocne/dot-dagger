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
  --env context=personal \
  --dry-run

! test -e /home/e2e/.zshrc                        || { printf 'FAIL: .zshrc should not exist after --dry-run\n'; exit 1; }
! test -e /tmp/xdgdata/dot-dagger/init.sh         || { printf 'FAIL: init.sh should not exist after --dry-run\n'; exit 1; }
! test -e /home/e2e/bin/dot-dagger/hello          || { printf 'FAIL: bin/hello should not exist after --dry-run\n'; exit 1; }

printf 'PASS: dry-run test\n'
