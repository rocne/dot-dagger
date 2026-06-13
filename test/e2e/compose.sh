#!/bin/sh
set -e

mkdir -p /home/e2e/bin /tmp/generated

dotd apply \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --link-root /home/e2e \
  --bin-dir /home/e2e/bin \
  --init-file /tmp/init.sh \
  --generated-dir /tmp/generated \
  --env os=linux \
  --env context=personal

test -f /tmp/generated/extras.sh || { printf 'FAIL: extras.sh not generated\n'; exit 1; }
grep -q "extras.sh" /tmp/init.sh || { printf 'FAIL: extras.sh not sourced in init.sh\n'; exit 1; }

OUT=$(dotd compose list \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --link-root /home/e2e \
  --bin-dir /home/e2e/bin \
  --init-file /tmp/init.sh \
  --generated-dir /tmp/generated \
  --env os=linux \
  --env context=personal)
printf '%s' "$OUT" | grep -q "extras.sh" \
  || { printf 'FAIL: compose list missing extras.sh\n'; exit 1; }

dotd compose check \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --link-root /home/e2e \
  --bin-dir /home/e2e/bin \
  --init-file /tmp/init.sh \
  --generated-dir /tmp/generated \
  --env os=linux \
  --env context=personal \
  || { printf 'FAIL: compose check should pass after apply\n'; exit 1; }

printf 'stale content\n' > /tmp/generated/extras.sh

# Stale target must be reported AND exit non-zero.
if OUT=$(dotd compose check \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --link-root /home/e2e \
  --bin-dir /home/e2e/bin \
  --init-file /tmp/init.sh \
  --generated-dir /tmp/generated \
  --env os=linux \
  --env context=personal 2>&1); then
  printf 'FAIL: compose check should exit non-zero when stale\n'; exit 1
fi
printf '%s' "$OUT" | grep -q "stale" \
  || { printf 'FAIL: compose check should report stale\n'; exit 1; }

printf 'PASS: compose test\n'
