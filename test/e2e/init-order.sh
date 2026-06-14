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

pos_base=$(grep -n "base.sh"    /tmp/xdgdata/dot-dagger/init.sh | head -1 | cut -d: -f1)
pos_path=$(grep -n "path.sh"    /tmp/xdgdata/dot-dagger/init.sh | head -1 | cut -d: -f1)
pos_aliases=$(grep -n "aliases.sh" /tmp/xdgdata/dot-dagger/init.sh | head -1 | cut -d: -f1)
pos_linux=$(grep -n "linux.sh"  /tmp/xdgdata/dot-dagger/init.sh | head -1 | cut -d: -f1)

[ -n "$pos_base" ]    || { printf 'FAIL: base.sh not in init.sh\n';    exit 1; }
[ -n "$pos_path" ]    || { printf 'FAIL: path.sh not in init.sh\n';    exit 1; }
[ -n "$pos_aliases" ] || { printf 'FAIL: aliases.sh not in init.sh\n'; exit 1; }
[ -n "$pos_linux" ]   || { printf 'FAIL: linux.sh not in init.sh\n';   exit 1; }

[ "$pos_base" -lt "$pos_path" ]    || { printf 'FAIL: base.sh must come before path.sh\n';    exit 1; }
[ "$pos_path" -lt "$pos_aliases" ] || { printf 'FAIL: path.sh must come before aliases.sh\n'; exit 1; }
[ "$pos_base" -lt "$pos_linux" ]   || { printf 'FAIL: base.sh must come before linux.sh\n';   exit 1; }

printf 'PASS: init-order test\n'
