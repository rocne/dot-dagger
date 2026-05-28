#!/bin/sh
set -e

mkdir -p /home/e2e/bin /tmp/generated

COMMON_ARGS="--files /fixture --env-file /fixture/env.yaml --link-root /home/e2e --bin-dir /home/e2e/bin --init-file /tmp/init.sh --generated-dir /tmp/generated --env os=linux --env context=personal"

dotd apply $COMMON_ARGS

dotd check $COMMON_ARGS \
  || { printf 'FAIL: dotd check failed after successful apply\n'; exit 1; }

printf 'PASS: check test\n'
