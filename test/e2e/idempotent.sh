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

cp /tmp/xdgdata/dot-dagger/init.sh /tmp/init1.sh

dotd apply \
  --files /fixture \
  --dotd-env /fixture/env.yaml \
  --env os=linux \
  --env context=personal

diff /tmp/init1.sh /tmp/xdgdata/dot-dagger/init.sh \
  || { printf 'FAIL: init.sh changed between two identical apply runs\n'; exit 1; }

printf 'PASS: idempotent test\n'
