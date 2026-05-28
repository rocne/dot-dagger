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
  --env context=personal \
  --dry-run

! test -e /home/e2e/.zshrc    || { printf 'FAIL: .zshrc should not exist after --dry-run\n'; exit 1; }
! test -e /tmp/init.sh        || { printf 'FAIL: init.sh should not exist after --dry-run\n'; exit 1; }
! test -e /home/e2e/bin/hello || { printf 'FAIL: bin/hello should not exist after --dry-run\n'; exit 1; }

printf 'PASS: dry-run test\n'
