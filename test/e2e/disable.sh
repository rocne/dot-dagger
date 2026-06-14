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

! grep -q "disabled.sh" /tmp/xdgdata/dot-dagger/init.sh \
  || { printf 'FAIL: disabled.sh should not appear in init.sh\n'; exit 1; }

grep -q "base.sh" /tmp/xdgdata/dot-dagger/init.sh \
  || { printf 'FAIL: base.sh missing from init.sh (control check)\n'; exit 1; }

printf 'PASS: disable test\n'
