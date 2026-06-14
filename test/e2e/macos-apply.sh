#!/bin/sh
set -e

export HOME=/home/e2e
export XDG_BIN_HOME=/home/e2e/bin
export XDG_DATA_HOME=/tmp/xdgdata
mkdir -p /home/e2e/bin /tmp/xdgdata

dotd apply \
  --files /fixture \
  --dotd-env /fixture/env.yaml \
  --env os=macos \
  --env context=personal

grep -q "macos.sh"   /tmp/xdgdata/dot-dagger/init.sh || { printf 'FAIL: macos.sh not in init.sh\n'; exit 1; }
! grep -q "linux.sh" /tmp/xdgdata/dot-dagger/init.sh || { printf 'FAIL: linux.sh should not be in init.sh for os=macos\n'; exit 1; }

printf 'PASS: macos-apply test\n'
