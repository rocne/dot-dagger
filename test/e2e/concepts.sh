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
mkdir -p /home/e2e/bin /tmp/generated
dotd --debug list \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --link-root /home/e2e \
  --bin-dir /home/e2e/bin \
  --init-file /tmp/init.sh \
  --generated-dir /tmp/generated \
  --env os=linux \
  --env context=personal \
  >/dev/null 2>/tmp/debug.log \
  || { printf 'FAIL: dotd --debug list exited non-zero\n'; exit 1; }
grep -q 'DEBU' /tmp/debug.log \
  || { printf 'FAIL: --debug produced no DEBU output\nLog:\n'; cat /tmp/debug.log; exit 1; }
printf 'PASS: --debug flag\n'
