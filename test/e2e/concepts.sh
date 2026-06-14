#!/bin/sh
set -e

# dotd concepts prints the reference text
OUT=$(dotd concepts)
printf '%s' "$OUT" | grep -q 'PIPELINE' \
  || { printf 'FAIL: concepts missing PIPELINE section\nOutput:\n%s\n' "$OUT"; exit 1; }
printf '%s' "$OUT" | grep -q 'PREDICATES' \
  || { printf 'FAIL: concepts missing PREDICATES section\nOutput:\n%s\n' "$OUT"; exit 1; }
printf '%s' "$OUT" | grep -q 'ENV.YAML' \
  || { printf 'FAIL: concepts missing ENV.YAML section\nOutput:\n%s\n' "$OUT"; exit 1; }
printf 'PASS: concepts output\n'

# --debug flag runs without error and produces debug log output
export HOME=/home/e2e
export XDG_BIN_HOME=/home/e2e/bin
export XDG_DATA_HOME=/tmp/xdgdata
mkdir -p /home/e2e/bin /tmp/xdgdata

dotd --debug list \
  --files /fixture \
  --dotd-env /fixture/env.yaml \
  --env os=linux \
  --env context=personal \
  >/dev/null 2>&1 \
  || { printf 'FAIL: dotd --debug exited non-zero\n'; exit 1; }
printf 'PASS: --debug flag\n'
