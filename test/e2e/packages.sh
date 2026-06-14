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

grep -q "needs-fake.sh" /tmp/xdgdata/dot-dagger/init.sh \
  || { printf 'FAIL: needs-fake.sh should be in init.sh (@require satisfied)\n'; exit 1; }

grep -q "optional-tool.sh" /tmp/xdgdata/dot-dagger/init.sh \
  || { printf 'FAIL: optional-tool.sh should be in init.sh (@request is soft)\n'; exit 1; }

printf 'PASS: packages test\n'
