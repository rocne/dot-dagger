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

dotd unapply --yes \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --link-root /home/e2e \
  --bin-dir /home/e2e/bin \
  --init-file /tmp/init.sh \
  --generated-dir /tmp/generated \
  --env os=linux \
  --env context=personal

test ! -L /home/e2e/.zshrc        || { printf 'FAIL: .zshrc symlink should be removed\n'; exit 1; }
test ! -f /tmp/init.sh             || { printf 'FAIL: init.sh should be removed\n'; exit 1; }
test ! -L /home/e2e/bin/hello      || { printf 'FAIL: bin/hello symlink should be removed\n'; exit 1; }

printf 'PASS: unapply test\n'
