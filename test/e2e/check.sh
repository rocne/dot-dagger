#!/bin/sh
set -e

export HOME=/home/e2e
export XDG_BIN_HOME=/home/e2e/bin
export XDG_DATA_HOME=/tmp/xdgdata
mkdir -p /home/e2e/bin /tmp/xdgdata

COMMON_ARGS="--files /fixture --dotd-env /fixture/env.yaml --env os=linux --env context=personal"

dotd apply $COMMON_ARGS

dotd check $COMMON_ARGS \
  || { printf 'FAIL: dotd check failed after successful apply\n'; exit 1; }

printf 'PASS: check test\n'
