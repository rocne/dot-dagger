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
  --env os=macos \
  --env context=personal

grep -q "macos.sh"   /tmp/init.sh || { printf 'FAIL: macos.sh not in init.sh\n'; exit 1; }
! grep -q "linux.sh" /tmp/init.sh || { printf 'FAIL: linux.sh should not be in init.sh for os=macos\n'; exit 1; }

printf 'PASS: macos-apply test\n'
