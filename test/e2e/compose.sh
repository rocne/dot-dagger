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

test -f /tmp/xdgdata/dot-dagger/generated/extras.sh || { printf 'FAIL: extras.sh not generated\n'; exit 1; }
grep -q "extras.sh" /tmp/xdgdata/dot-dagger/init.sh || { printf 'FAIL: extras.sh not sourced in init.sh\n'; exit 1; }

OUT=$(dotd compose list \
  --files /fixture \
  --dotd-env /fixture/env.yaml \
  --env os=linux \
  --env context=personal)
printf '%s' "$OUT" | grep -q "extras.sh" \
  || { printf 'FAIL: compose list missing extras.sh\n'; exit 1; }

dotd compose check \
  --files /fixture \
  --dotd-env /fixture/env.yaml \
  --env os=linux \
  --env context=personal \
  || { printf 'FAIL: compose check should pass after apply\n'; exit 1; }

printf 'stale content\n' > /tmp/xdgdata/dot-dagger/generated/extras.sh

# Stale target must be reported AND exit non-zero.
if OUT=$(dotd compose check \
  --files /fixture \
  --dotd-env /fixture/env.yaml \
  --env os=linux \
  --env context=personal 2>&1); then
  printf 'FAIL: compose check should exit non-zero when stale\n'; exit 1
fi
printf '%s' "$OUT" | grep -q "stale" \
  || { printf 'FAIL: compose check should report stale\n'; exit 1; }

printf 'PASS: compose test\n'
