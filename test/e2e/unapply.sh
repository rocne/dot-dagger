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

dotd unapply --yes \
  --files /fixture \
  --dotd-env /fixture/env.yaml \
  --env os=linux \
  --env context=personal

test ! -L /home/e2e/.zshrc                    || { printf 'FAIL: .zshrc symlink should be removed\n'; exit 1; }
test ! -f /tmp/xdgdata/dot-dagger/init.sh     || { printf 'FAIL: init.sh should be removed\n'; exit 1; }
test ! -L /home/e2e/bin/dot-dagger/hello      || { printf 'FAIL: bin/hello symlink should be removed\n'; exit 1; }

printf 'PASS: unapply test\n'
