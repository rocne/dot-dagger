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

printf 'n\n' | dotd unapply \
  --files /fixture \
  --dotd-env /fixture/env.yaml \
  --env os=linux \
  --env context=personal

test -L /home/e2e/.zshrc || { printf 'FAIL: .zshrc symlink should be preserved on cancel\n'; exit 1; }

printf 'PASS: unapply-cancel test\n'
